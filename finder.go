package siteperf

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"net/url"
	"runtime"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bounoable/siteperf/internal/plog"
	"github.com/gocolly/colly"
)

type Finder struct {
	rootURL   *url.URL
	pageLimit int
	log       *slog.Logger
}

func New(rootURL string, pageLimit int) (*Finder, error) {
	u, err := url.Parse(rootURL)
	if err != nil {
		return nil, err
	}
	return &Finder{
		rootURL:   u,
		pageLimit: pageLimit,
		log:       plog.New("Finder"),
	}, nil
}

func (f *Finder) FindUnused(ctx context.Context, classes []string) ([]string, error) {
	usedChan, err := f.findUsed(ctx)
	if err != nil {
		return nil, fmt.Errorf("find used classes: %w", err)
	}

	used, err := drain(usedChan)
	if err != nil {
		return nil, fmt.Errorf("find used classes: %w", err)
	}

	unused := filter(classes, func(s string) bool {
		return !slices.ContainsFunc(used, func(uc usedClass) bool {
			return uc.class == s && uc.count > 0
		})
	})

	return unused, nil
}

type usedClass struct {
	class string
	count int
}

func (f *Finder) findUsed(ctx context.Context) (<-chan usedClass, error) {
	c := colly.NewCollector(colly.Async(true))
	c.Limit(&colly.LimitRule{
		Delay:       150 * time.Millisecond,
		RandomDelay: 50 * time.Millisecond,
		Parallelism: int(math.Min(8, float64(runtime.NumCPU()))),
	})

	classChan := make(chan string)
	var (
		classes    []string
		classCount = make(map[string]int)
	)
	classesDone := make(chan struct{})

	go func() {
		defer close(classesDone)
		for class := range classChan {
			if classCount[class] == 0 {
				classes = append(classes, class)
			}
			classCount[class]++
		}
	}()

	c.OnHTML("[class]", func(e *colly.HTMLElement) {
		classList := strings.Split(e.Attr("class"), " ")
		classList = filter(classList, func(s string) bool {
			return strings.TrimSpace(s) != ""
		})

		for _, class := range classList {
			select {
			case <-ctx.Done():
				return
			case classChan <- class:
			}
		}
	})

	var visitCount atomic.Uint64
	var visitedPaths sync.Map
	c.OnHTML("a[href]", func(e *colly.HTMLElement) {
		if f.pageLimit > 0 {
			if count := visitCount.Load(); count > uint64(f.pageLimit) {
				return
			}
		}

		href := e.Attr("href")
		to, err := url.Parse(href)
		if err != nil {
			return
		}

		visited, ok := visitedPaths.Load(to.Path)
		if ok && visited.(bool) {
			return
		}

		if to.Host != f.rootURL.Host {
			return
		}

		e.Request.Visit(e.Attr("href"))
		visitCount.Add(1)
		visitedPaths.Store(to.Path, true)

		f.log.Debug("Visited page", "total", visitCount.Load())
	})

	c.OnRequest(func(r *colly.Request) { f.log.Info("Visiting page", "url", r.URL.String()) })

	if err := c.Visit(f.rootURL.String()); err != nil {
		return nil, fmt.Errorf("visit root URL: %w", err)
	}

	go func() {
		c.Wait()
		close(classChan)
	}()

	usedClasses := make(chan usedClass)
	go func() {
		defer close(usedClasses)
		<-classesDone

		for _, class := range classes {
			usedClasses <- usedClass{
				class: class,
				count: classCount[class],
			}
		}
	}()

	return usedClasses, nil
}

func filter[S ~[]E, E any](s S, fn func(E) bool) S {
	if s == nil {
		return nil
	}
	filtered := make(S, 0, len(s))
	for _, e := range s {
		if fn(e) {
			filtered = append(filtered, e)
		}
	}
	return filtered
}

func drain[C <-chan E, E any](c C) ([]E, error) {
	var elems []E
	for elem := range c {
		elems = append(elems, elem)
	}
	return elems, nil
}
