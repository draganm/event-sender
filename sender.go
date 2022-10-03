package eventsender

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/go-logr/logr"
	"golang.org/x/sync/errgroup"
)

type EventSender struct {
	ctx      context.Context
	endpoint string
	queue    chan interface{}
	log      logr.Logger
}

var ErrEventDropped = errors.New("event was not sent")

func New(ctx context.Context, endpoint string, queueSize int) (*EventSender, error) {
	_, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("could not parse endpoint URL: %w", err)
	}

	eg, ctx := errgroup.WithContext(ctx)

	logr.Discard()
	log := logr.FromContextOrDiscard(ctx)

	queue := make(chan interface{}, queueSize)

	e := &EventSender{ctx: ctx, endpoint: endpoint, queue: queue, log: log}

	eg.Go(e.senderProcess)

	return e, nil
}

const maxBatchSize = 100

func (e *EventSender) senderProcess() error {
	for e.ctx.Err() == nil {
		events := []interface{}{}
		evt := <-e.queue
		events = append(events, evt)
	loop:
		for len(events) < maxBatchSize {
			select {
			case evt = <-e.queue:
				events = append(events, evt)
				continue
			default:
				break loop
			}
		}

		for ; ; time.Sleep(300 * time.Millisecond) {
			d, err := json.Marshal(events)
			if err != nil {
				e.log.Error(err, "could not serialize events")
				break
			}

			if len(events) != 0 {
				req, err := http.NewRequest("POST", e.endpoint, bytes.NewReader(d))
				if err != nil {
					e.log.Error(err, "could not create post request")
					continue
				}

				res, err := http.DefaultClient.Do(req)
				if err != nil {
					e.log.Error(err, "could not perform post request")
					continue
				}

				res.Body.Close()

				if res.StatusCode != 200 {
					e.log.Error(fmt.Errorf("unexpected status %s, expected 200", res.Status), "unexpected response")
					continue
				}

			}

			break

		}

		events = nil

	}

	return e.ctx.Err()
}

func (e *EventSender) SendEvent(evt interface{}) error {
	select {
	case e.queue <- evt:
		return nil
	default:
		e.log.Info("dropped event because send queue is full")
		return ErrEventDropped
	}

}
