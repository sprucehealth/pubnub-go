package e2e

import (
	"fmt"
	"io/ioutil"
	"log"
	"strings"
	"sync"
	"testing"
	"time"

	pubnub "github.com/pubnub/go"
	"github.com/pubnub/go/tests/stubs"
	"github.com/stretchr/testify/assert"
)

func TestHeartbeatTimeoutEvent(t *testing.T) {
	assert := assert.New(t)
	ch := randomized("hb-te")
	emitterUuid := randomized("emitter")

	var wg sync.WaitGroup
	wg.Add(2)

	donePresenceConnect := make(chan bool)
	doneJoin := make(chan bool)
	doneTimeout := make(chan bool)
	errChan := make(chan string)

	configEmitter := configCopy()
	configEmitter.SetPresenceTimeout(6)

	configPresenceListener := configCopy()

	configEmitter.Uuid = emitterUuid
	configPresenceListener.Uuid = randomized("listener")

	pn := pubnub.NewPubNub(configEmitter)
	pnPresenceListener := pubnub.NewPubNub(configPresenceListener)

	listenerEmitter := pubnub.NewListener()
	listenerPresenceListener := pubnub.NewListener()

	// emitter
	go func() {
		for {
			select {
			case status := <-listenerEmitter.Status:
				switch status.Category {
				case pubnub.PNConnectedCategory:
					wg.Done()
					return
				}
			case <-listenerEmitter.Message:
				errChan <- "Got message while awaiting for a status event"
				return
			case <-listenerEmitter.Presence:
				errChan <- "Got presence while awaiting for a status event"
				return
			}
		}
	}()

	// listener
	go func() {
		for {
			select {
			case status := <-listenerPresenceListener.Status:
				switch status.Category {
				case pubnub.PNConnectedCategory:
					donePresenceConnect <- true
				}
			case message := <-listenerPresenceListener.Message:
				errChan <- fmt.Sprintf("Unexpected message: %s",
					message.Message)
			case presence := <-listenerPresenceListener.Presence:
				// ignore join event of presence listener
				if presence.Uuid == configPresenceListener.Uuid {
					continue
				}

				assert.Equal(ch, presence.Channel)

				if presence.Event == "timeout" {
					assert.Equal(configEmitter.Uuid, presence.Uuid)
					doneTimeout <- true
					return
				} else if presence.Event == "join" {
					assert.Equal("join", presence.Event)
					assert.Equal(configEmitter.Uuid, presence.Uuid)
					wg.Done()
				}
			}
		}
	}()

	pn.AddListener(listenerEmitter)
	pnPresenceListener.AddListener(listenerPresenceListener)

	pnPresenceListener.Subscribe(&pubnub.SubscribeOperation{
		Channels:        []string{ch},
		PresenceEnabled: true,
	})

	select {
	case <-donePresenceConnect:
	case err := <-errChan:
		assert.Fail(err)
		return
	}

	pn.Subscribe(&pubnub.SubscribeOperation{
		Channels: []string{ch},
	})

	go func() {
		wg.Wait()
		doneJoin <- true
	}()

	select {
	case <-doneJoin:
	case err := <-errChan:
		assert.Fail(err)
		return
	}

	cl := pubnub.NewHttpClient(15, 15)
	cl.Transport = fakeTransport{
		Status:     "200 OK",
		StatusCode: 200,
		// WARNING: can be read only once
		Body: ioutil.NopCloser(strings.NewReader("Hijacked response")),
	}
	pn.SetClient(cl)

	select {
	case <-doneTimeout:
		pn.UnsubscribeAll()
	case <-time.After(8 * time.Second):
		assert.Fail("No timeout event received in 8 seconds")
	case err := <-errChan:
		assert.Fail(err)
		return
	}
}

func TestHeartbeatStubbedRequest(t *testing.T) {
	assert := assert.New(t)
	interceptor := stubs.NewInterceptor()
	interceptor.AddStub(&stubs.Stub{
		Method:             "GET",
		Path:               "/v2/presence/sub-key/sub-c-5c4fdcc6-c040-11e5-a316-0619f8945a4f/channel/ch/heartbeat",
		Query:              "heartbeat=6",
		ResponseBody:       "{\"status\": 200, \"message\": \"OK\", \"service\": \"Presence\"}",
		IgnoreQueryKeys:    []string{"uuid", "pnsdk"},
		ResponseStatusCode: 200,
	})

	interceptor.AddStub(&stubs.Stub{
		Method:             "GET",
		Path:               "/v2/presence/sub-key/sub-c-5c4fdcc6-c040-11e5-a316-0619f8945a4f/channel/ch/leave",
		Query:              "",
		ResponseBody:       "{\"status\": 200, \"message\": \"OK\", \"service\": \"Presence\", \"action\": \"leave\"}",
		IgnoreQueryKeys:    []string{"uuid", "pnsdk"},
		ResponseStatusCode: 200,
	})

	doneConnect := make(chan bool)
	doneHeartbeat := make(chan bool)
	errChan := make(chan string)

	config := configCopy()
	config.SetPresenceTimeout(6)

	pn := pubnub.NewPubNub(config)

	listener := pubnub.NewListener()
	pn.AddListener(listener)

	go func() {
		for {
			select {
			case status := <-listener.Status:
				switch status.Operation {
				case pubnub.PNHeartBeatOperation:
					doneHeartbeat <- true
				}

				switch status.Category {
				case pubnub.PNConnectedCategory:
					doneConnect <- true
				}
			case <-listener.Message:
				errChan <- "Got message while awaiting for a status event"
				return
			case <-listener.Presence:
				errChan <- "Got presence while awaiting for a status event"
				return
			}
		}
	}()

	pn.Subscribe(&pubnub.SubscribeOperation{
		Channels: []string{"ch"},
	})

	select {
	case <-doneConnect:
	case err := <-errChan:
		assert.Fail(err)
	}

	pn.SetClient(interceptor.GetClient())

	select {
	case <-doneHeartbeat:
	case <-time.After(3 * time.Second):
		assert.Fail("Heartbeat status was expected")
	}

	pn.Unsubscribe(&pubnub.UnsubscribeOperation{
		Channels: []string{"ch"},
	})
}

// Test triggers BadRequestCategory in subscription.Status channel
// for failed HB call
func TestHeartbeatRequestWithError(t *testing.T) {
	assert := assert.New(t)
	interceptor := stubs.NewInterceptor()
	interceptor.AddStub(&stubs.Stub{
		Method:             "GET",
		Path:               "/v2/presence/sub-key/sub-c-5c4fdcc6-c040-11e5-a316-0619f8945a4f/channel/ch/heartbeat",
		Query:              "heartbeat=6",
		ResponseBody:       "{\"status\": 404, \"message\": \"Not Found\", \"error\": \"1\", \"service\": \"Presence\"}",
		IgnoreQueryKeys:    []string{"uuid", "pnsdk"},
		ResponseStatusCode: 404,
	})

	interceptor.AddStub(&stubs.Stub{
		Method:             "GET",
		Path:               "/v2/presence/sub-key/sub-c-5c4fdcc6-c040-11e5-a316-0619f8945a4f/channel/ch/leave",
		Query:              "",
		ResponseBody:       "{\"status\": 200, \"message\": \"OK\", \"service\": \"Presence\", \"action\": \"leave\"}",
		IgnoreQueryKeys:    []string{"uuid", "pnsdk"},
		ResponseStatusCode: 200,
	})

	doneConnect := make(chan bool)
	doneHeartbeat := make(chan bool)
	errChan := make(chan string)

	config := configCopy()
	config.SetPresenceTimeout(6)

	pn := pubnub.NewPubNub(config)

	listener := pubnub.NewListener()
	pn.AddListener(listener)

	go func() {
		for {
			select {
			case status := <-listener.Status:
				switch status.Category {
				case pubnub.PNConnectedCategory:
					doneConnect <- true
				case pubnub.PNBadRequestCategory:
					doneHeartbeat <- true
				}
			case <-listener.Message:
				errChan <- "Got message while awaiting for a status event"
				return
			case <-listener.Presence:
				errChan <- "Got presence while awaiting for a status event"
				return
			}
		}
	}()

	pn.Subscribe(&pubnub.SubscribeOperation{
		Channels: []string{"ch"},
	})

	select {
	case <-doneConnect:
	case err := <-errChan:
		assert.Fail(err)
	}

	pn.SetClient(interceptor.GetClient())

	select {
	case <-doneHeartbeat:
	case <-time.After(3 * time.Second):
		assert.Fail("Heartbeat status was expected")
	}

	pn.Unsubscribe(&pubnub.UnsubscribeOperation{
		Channels: []string{"ch"},
	})
}

// NOTICE: snippet for manual hb testing
// - subscribe 'first'
// - unsubscribeAll
// - subscribe 'first'
// - subscribe 'second'
// - unsubscribe 'first', 'second'
func xTestHeartbeatRandomizedBehaviour(t *testing.T) {
	assert := assert.New(t)
	first := "first"
	second := "second"
	emitterUuid := randomized("emitter")

	var wg sync.WaitGroup
	wg.Add(2)

	doneJoin := make(chan bool)
	doneTimeout := make(chan bool)
	errChan := make(chan string)

	configEmitter := configCopy()
	configEmitter.SetPresenceTimeout(6)

	configEmitter.Uuid = emitterUuid

	pn := pubnub.NewPubNub(configEmitter)

	listenerEmitter := pubnub.NewListener()

	// emitter
	go func() {
		for {
			select {
			case status := <-listenerEmitter.Status:
				switch status.Category {
				case pubnub.PNConnectedCategory:
					doneJoin <- true
					return
				}
			case <-listenerEmitter.Message:
				errChan <- "Got message while awaiting for a status event"
				return
			case <-listenerEmitter.Presence:
				errChan <- "Got presence while awaiting for a status event"
				return
			}
		}
	}()

	pn.AddListener(listenerEmitter)

	pn.Subscribe(&pubnub.SubscribeOperation{
		Channels: []string{first},
	})

	go func() {
		doneJoin <- true
	}()

	select {
	case <-doneJoin:
	case err := <-errChan:
		assert.Fail(err)
		return
	}

	log.Println("Sleeping 8s")
	time.Sleep(8 * time.Second)

	pn.UnsubscribeAll()

	log.Println("Unsubscribed")
	log.Println("Sleeping 8s")
	time.Sleep(8 * time.Second)

	pn.Subscribe(&pubnub.SubscribeOperation{
		Channels: []string{first},
	})

	log.Println("Subscribed again")
	log.Println("Sleeping 8s")
	time.Sleep(8 * time.Second)

	pn.Subscribe(&pubnub.SubscribeOperation{
		Channels: []string{second},
	})

	log.Println("Subsccribed to another channel again")
	log.Println("Sleeping 8s")
	time.Sleep(8 * time.Second)

	pn.Unsubscribe(&pubnub.UnsubscribeOperation{
		Channels: []string{first, second},
	})

	log.Println("Unsubscribed")
	log.Println("Sleeping 8s")
	time.Sleep(8 * time.Second)

	select {
	case <-doneTimeout:
	case <-time.After(8 * time.Second):
		assert.Fail("No timeout event received in 8 seconds")
	case err := <-errChan:
		assert.Fail(err)
		return
	}
}