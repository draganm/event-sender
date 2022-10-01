package eventsender_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	eventsender "github.com/draganm/event-sender"
	"github.com/prysmaticlabs/prysm/shared/testutil/require"
)

func TestSender(t *testing.T) {

	recv := make(chan interface{}, 1)

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		dec := json.NewDecoder(r.Body)
		dat := []interface{}{}
		err := dec.Decode(&dat)
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}

		recv <- dat

	}))

	defer testServer.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	es, err := eventsender.New(ctx, testServer.URL, 10)
	require.NoError(t, err)

	err = es.SendEvent("abc")

	require.NoError(t, err)
	require.DeepSSZEqual(t, []interface{}{"abc"}, <-recv)

}
