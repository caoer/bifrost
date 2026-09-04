package integrations

import (
	"io"
	"testing"
	"testing/synctest"
	"time"

	bifrost "github.com/maximhq/bifrost/core"
	"github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/transports/bifrost-http/lib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// Test_streamWithEarlyCommitCommitsSSEWhileSetupPends: a stream setup that is
// still pending past streamEarlyCommitAfter must have the response committed as
// SSE and carrying heartbeats before the upstream answers, and the upstream's
// eventual error must reach the client as the route's SSE error frame.
func Test_streamWithEarlyCommitCommitsSSEWhileSetupPends(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		router := NewGenericRouter(nil, &mockHandlerStore{}, nil, nil, bifrost.NewNoOpLogger())
		ctx := &fasthttp.RequestCtx{}
		release := make(chan struct{})
		start := func() (chan *schemas.BifrostStreamChunk, *schemas.BifrostError) {
			<-release
			return nil, newBifrostErrorWithCode(nil, "upstream gave up", fasthttp.StatusBadGateway)
		}
		config := RouteConfig{
			Type: RouteConfigTypeAnthropic,
			StreamConfig: &StreamConfig{
				ErrorConverter: func(_ *schemas.BifrostContext, err *schemas.BifrostError) interface{} {
					return "event: error\ndata: {\"message\":\"" + err.Error.Message + "\"}\n\n"
				},
			},
		}

		handlerDone := make(chan struct{})
		go func() {
			router.streamWithEarlyCommit(ctx, nil, config, schemas.ChatCompletionStreamRequest, start, func() {})
			close(handlerDone)
		}()

		time.Sleep(streamEarlyCommitAfter + time.Millisecond)
		<-handlerDone
		assert.Equal(t, "text/event-stream", string(ctx.Response.Header.ContentType()), "response must be committed as SSE before the upstream answers")
		assert.Equal(t, fasthttp.StatusOK, ctx.Response.StatusCode())

		type readResult struct {
			body string
			err  error
		}
		readDone := make(chan readResult, 1)
		bodyStream := ctx.Response.BodyStream()
		require.NotNil(t, bodyStream)
		go func() {
			b, err := io.ReadAll(bodyStream)
			readDone <- readResult{body: string(b), err: err}
		}()

		time.Sleep(2*lib.DefaultSSEHeartbeatInterval + time.Millisecond)
		close(release)

		result := <-readDone
		require.NoError(t, result.err)
		assert.Contains(t, result.body, ": heartbeat\n", "heartbeats must flow while the setup is pending")
		assert.Contains(t, result.body, "event: error\ndata: {\"message\":\"upstream gave up\"}\n\n", "the late setup error must arrive as the route's SSE error frame")
	})
}

// Test_streamWithEarlyCommitKeepsHTTPStatusForPromptErrors: a setup that fails
// within streamEarlyCommitAfter keeps the pre-existing behaviour — an HTTP error
// status, no SSE commit.
func Test_streamWithEarlyCommitKeepsHTTPStatusForPromptErrors(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		router := NewGenericRouter(nil, &mockHandlerStore{}, nil, nil, bifrost.NewNoOpLogger())
		ctx := &fasthttp.RequestCtx{}
		start := func() (chan *schemas.BifrostStreamChunk, *schemas.BifrostError) {
			return nil, newBifrostErrorWithCode(nil, "upstream gave up", fasthttp.StatusBadGateway)
		}
		config := RouteConfig{
			Type:         RouteConfigTypeAnthropic,
			StreamConfig: &StreamConfig{},
			ErrorConverter: func(_ *schemas.BifrostContext, err *schemas.BifrostError) interface{} {
				return map[string]string{"message": err.Error.Message}
			},
		}

		router.streamWithEarlyCommit(ctx, nil, config, schemas.ChatCompletionStreamRequest, start, func() {})

		assert.Equal(t, fasthttp.StatusBadGateway, ctx.Response.StatusCode())
		assert.NotEqual(t, "text/event-stream", string(ctx.Response.Header.ContentType()))
	})
}
