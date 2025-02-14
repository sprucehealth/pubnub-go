package pubnub

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/sprucehealth/pubnub-go/pnerr"
	"github.com/sprucehealth/pubnub-go/utils"
)

const listChannelsOfPushPath = "/v1/push/sub-key/%s/devices/%s"

var emptyListPushProvisionsRequestResponse *ListPushProvisionsRequestResponse

type listPushProvisionsRequestBuilder struct {
	opts *listPushProvisionsRequestOpts
}

func newListPushProvisionsRequestBuilder(pubnub *PubNub) *listPushProvisionsRequestBuilder {
	builder := listPushProvisionsRequestBuilder{
		opts: &listPushProvisionsRequestOpts{
			pubnub: pubnub,
		},
	}

	return &builder
}

func newListPushProvisionsRequestBuilderWithContext(
	pubnub *PubNub, context Context) *listPushProvisionsRequestBuilder {
	builder := listPushProvisionsRequestBuilder{
		opts: &listPushProvisionsRequestOpts{
			pubnub: pubnub,
			ctx:    context,
		},
	}

	return &builder
}

// PushType sets the PushType for the List Push Provisions request.
func (b *listPushProvisionsRequestBuilder) PushType(
	pushType PNPushType) *listPushProvisionsRequestBuilder {
	b.opts.PushType = pushType
	return b
}

// DeviceIDForPush sets the device id for List Push Provisions request.
func (b *listPushProvisionsRequestBuilder) DeviceIDForPush(
	deviceID string) *listPushProvisionsRequestBuilder {
	b.opts.DeviceIDForPush = deviceID
	return b
}

// QueryParam accepts a map, the keys and values of the map are passed as the query string parameters of the URL called by the API.
func (b *listPushProvisionsRequestBuilder) QueryParam(queryParam map[string]string) *listPushProvisionsRequestBuilder {
	b.opts.QueryParam = queryParam

	return b
}

// Execute runs the List Push Provisions request.
func (b *listPushProvisionsRequestBuilder) Execute() (
	*ListPushProvisionsRequestResponse, StatusResponse, error) {
	rawJSON, status, err := executeRequest(b.opts)
	if err != nil {
		return emptyListPushProvisionsRequestResponse, status, err
	}

	return newListPushProvisionsRequestResponse(rawJSON, status)
}

func newListPushProvisionsRequestResponse(jsonBytes []byte, status StatusResponse) (
	*ListPushProvisionsRequestResponse, StatusResponse, error) {
	resp := &ListPushProvisionsRequestResponse{}

	var value interface{}

	err := json.Unmarshal(jsonBytes, &value)
	if err != nil {
		e := pnerr.NewResponseParsingError("Error unmarshalling response",
			ioutil.NopCloser(bytes.NewBufferString(string(jsonBytes))), err)

		return emptyListPushProvisionsRequestResponse, status, e
	}

	if parsedValue, ok := value.([]interface{}); ok {
		a := make([]string, len(parsedValue))
		for i, v := range parsedValue {
			a[i] = v.(string)
		}
		resp.Channels = a
	}

	return resp, status, nil
}

type listPushProvisionsRequestOpts struct {
	pubnub *PubNub

	PushType PNPushType

	DeviceIDForPush string
	QueryParam      map[string]string
	Transport       http.RoundTripper

	ctx Context
}

func (o *listPushProvisionsRequestOpts) config() Config {
	return *o.pubnub.Config
}

func (o *listPushProvisionsRequestOpts) client() *http.Client {
	return o.pubnub.GetClient()
}

func (o *listPushProvisionsRequestOpts) context() Context {
	return o.ctx
}

func (o *listPushProvisionsRequestOpts) validate() error {
	if o.config().SubscribeKey == "" {
		return newValidationError(o, StrMissingSubKey)
	}

	if o.DeviceIDForPush == "" {
		return newValidationError(o, StrMissingDeviceID)
	}

	if o.PushType == PNPushTypeNone {
		return newValidationError(o, StrMissingPushType)
	}

	return nil
}

// ListPushProvisionsRequestResponse is the struct returned when the Execute function of ListPushProvisions is called.
type ListPushProvisionsRequestResponse struct {
	Channels []string
}

func (o *listPushProvisionsRequestOpts) buildPath() (string, error) {
	return fmt.Sprintf(listChannelsOfPushPath,
		o.pubnub.Config.SubscribeKey,
		utils.URLEncode(o.DeviceIDForPush)), nil
}

func (o *listPushProvisionsRequestOpts) buildQuery() (*url.Values, error) {
	q := defaultQuery(o.pubnub.Config.UUID, o.pubnub.telemetryManager)
	q.Set("type", o.PushType.String())
	SetQueryParam(q, o.QueryParam)
	return q, nil
}

func (o *listPushProvisionsRequestOpts) jobQueue() chan *JobQItem {
	return o.pubnub.jobQueue
}

func (o *listPushProvisionsRequestOpts) buildBody() ([]byte, error) {
	return []byte{}, nil
}

func (o *listPushProvisionsRequestOpts) httpMethod() string {
	return "GET"
}

func (o *listPushProvisionsRequestOpts) isAuthRequired() bool {
	return true
}

func (o *listPushProvisionsRequestOpts) requestTimeout() int {
	return o.pubnub.Config.NonSubscribeRequestTimeout
}

func (o *listPushProvisionsRequestOpts) connectTimeout() int {
	return o.pubnub.Config.ConnectTimeout
}

func (o *listPushProvisionsRequestOpts) operationType() OperationType {
	return PNRemoveGroupOperation
}

func (o *listPushProvisionsRequestOpts) telemetryManager() *TelemetryManager {
	return o.pubnub.telemetryManager
}
