package main

import (
	"context"
	"fmt"
	"sync"
)

type Request struct {
	Key       string `json:"key"`
	Reference `json:"reference"`
}

func streamBatch(ctx context.Context, requests []Request, resolve func(context.Context, int) (fileAPI, error), emit func(string, []byte)) error {
	if len(requests) == 0 || len(requests) > 12 {
		return fmt.Errorf("invalid visible thumbnail batch")
	}
	seen := map[string]bool{}
	for _, request := range requests {
		if request.Key == "" || len(request.Key) > 2048 || seen[request.Key] {
			return fmt.Errorf("invalid thumbnail key")
		}
		seen[request.Key] = true
	}
	jobs := make(chan Request)
	var workers sync.WaitGroup
	var output sync.Mutex
	for i := 0; i < 4; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for request := range jobs {
				if ctx.Err() != nil {
					return
				}
				api, err := resolve(ctx, request.DCID)
				if err != nil {
					continue
				}
				image, err := fetch(ctx, api, request.Reference)
				if err != nil || ctx.Err() != nil {
					continue
				}
				output.Lock()
				if ctx.Err() == nil {
					emit(request.Key, image)
				}
				output.Unlock()
			}
		}()
	}
dispatch:
	for _, request := range requests {
		select {
		case jobs <- request:
		case <-ctx.Done():
			break dispatch
		}
	}
	close(jobs)
	workers.Wait()
	return ctx.Err()
}
