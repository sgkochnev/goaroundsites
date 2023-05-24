package src

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
)

const (
	// You can limit concurrent net request. It's optional
	MaxGoroutines = 7
	// timeout for net requests
	Timeout = 2 * time.Second

	UserAgent = "Mozilla/5.0 (Windows NT 10.0) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/111.0.0.0 Safari/537.36"
)

type SiteStatus struct {
	Name          string
	StatusCode    int
	TimeOfRequest time.Time
}

type Monitor struct {
	StatusMap        map[string]SiteStatus
	Mtx              *sync.Mutex
	G                errgroup.Group
	Sites            []string
	RequestFrequency time.Duration
}

func NewMonitor(sites []string, requestFrequency time.Duration) *Monitor {
	return &Monitor{
		StatusMap:        make(map[string]SiteStatus),
		Mtx:              &sync.Mutex{},
		Sites:            sites,
		RequestFrequency: requestFrequency,
	}
}

func (m *Monitor) Run(ctx context.Context) error {
	// Run printStatuses(ctx) and checkSite(ctx) for m.Sites
	// Renew sites requests to map every m.RequestFrequency
	// Return if context closed

	m.G.SetLimit(MaxGoroutines)

	m.G.Go(func() error {
		return m.printStatuses(ctx)
	})

	m.G.Go(func() error {
		for {
			for _, site := range m.Sites {
				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
					site := site
					m.G.Go(func() error {
						m.checkSite(ctx, site)
						return nil
					})
				}
			}
			time.Sleep(m.RequestFrequency)
		}
	})

	return m.G.Wait()
}

func (m *Monitor) checkSite(ctx context.Context, site string) {
	// with http client go through site and write result to m.StatusMap
	client := http.Client{
		Timeout: Timeout,
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, site, nil)
	if err != nil {
		return
	}
	req.Header.Set("User-Agent", UserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return
	}
	_ = resp.Body.Close()

	m.Mtx.Lock()
	m.StatusMap[site] = SiteStatus{
		Name:          site,
		StatusCode:    resp.StatusCode,
		TimeOfRequest: time.Now(),
	}
	m.Mtx.Unlock()

}

func (m *Monitor) printStatuses(ctx context.Context) error {
	// print results of m.Status every second of until ctx cancelled
	tick := time.NewTicker(time.Second)
	defer tick.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tick.C:
			m.Mtx.Lock()
			for k, v := range m.StatusMap {
				_, err := fmt.Printf("%s : %+v\n", k, v)
				if err != nil {
					return err
				}
			}
			fmt.Println()
			m.Mtx.Unlock()
		}
	}
}
