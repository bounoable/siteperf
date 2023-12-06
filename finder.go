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
	"time"

	"github.com/bounoable/siteperf/internal/plog"
	"github.com/go-rod/rod"
)

// Finder locates unused CSS classes within a website starting from a given URL
// up to a specified limit of pages. It concurrently crawls the site, respects
// cancellation via context, and logs its progress using a structured logger.
// Finder provides the ability to identify classes that are not being utilized
// in any of the visited pages, helping in the optimization and cleanup of CSS
// resources. It operates with a customizable degree of concurrency, determined
// by available CPU resources, to efficiently process multiple pages in
// parallel.
type Finder struct {
	rootURL   *url.URL
	pageLimit int
	log       *slog.Logger
}

// New initializes a new Finder with the specified root URL and page limit,
// logging under the "Finder" namespace. It returns a pointer to the newly
// created Finder and any error that occurred during its creation, such as an
// invalid root URL.
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

// FindUnused identifies which of the provided CSS class names are not being
// used across the web pages within the scope defined by the root URL of the
// Finder instance. It traverses the website, starting from the root URL, and
// returns a slice of strings representing class names that appear to be unused.
// If an error occurs during the search process, it also returns an error
// detailing what went wrong.
func (f *Finder) FindUnused(ctx context.Context, classes []string) ([]string, error) {
	used, err := f.findUsed(ctx)
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

func (f *Finder) findUsed(ctx context.Context) ([]usedClass, error) {
	browser := rod.New().Context(ctx).MustConnect()
	defer browser.MustClose()

	workers := int(math.Min(4, float64(runtime.NumCPU())))
	var wg sync.WaitGroup
	wg.Add(workers)

	visited := visitedPages{paths: make(map[string]bool)}
	queue := make(chan string)
	enqueue := func(urls ...*url.URL) {
		for _, url := range urls {
			select {
			case <-ctx.Done():
				return
			case queue <- url.String():
			}
		}
	}

	classChan := make(chan usedClass)

	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()

			for {
				timer := time.NewTimer(10 * time.Second)

				select {
				case <-ctx.Done():
					timer.Stop()
					return
				case <-timer.C:
					timer.Stop()
					return
				case pageUrl := <-queue:
					timer.Stop()

					f.log.Debug("Visiting page", "url", pageUrl)

					page := browser.MustPage(pageUrl)

					if err := page.WaitLoad(); err != nil {
						f.log.Warn("Failed to load page", "url", pageUrl, "err", err)
						continue
					}

					if err := page.WaitStable(100 * time.Millisecond); err != nil {
						f.log.Warn("Failed to wait for page stability", "url", pageUrl, "err", err)
						continue
					}

					pageClasses, err := f.extractClasses(page, pageUrl)
					if err != nil {
						f.log.Warn("Failed to extract classes", "url", pageUrl, "err", err)
						continue
					}

					links, err := f.findLinks(page, pageUrl, &visited)
					if err != nil {
						f.log.Warn("Failed to find links", "url", pageUrl, "err", err)
						continue
					}

					go enqueue(links...)

					for _, class := range pageClasses {
						select {
						case <-ctx.Done():
							return
						case classChan <- class:
						}
					}
				}
			}
		}()
	}

	go enqueue(f.rootURL)

	go func() {
		wg.Wait()
		close(classChan)
	}()

	tmp := make(map[string]usedClass)
	for class := range classChan {
		tmp[class.class] = usedClass{
			class: class.class,
			count: tmp[class.class].count + class.count,
		}
	}

	out := make([]usedClass, 0, len(tmp))
	for _, class := range tmp {
		out = append(out, class)
	}

	return out, nil
}

func (f *Finder) findLinks(page *rod.Page, pageUrl string, visited *visitedPages) ([]*url.URL, error) {
	links, err := page.Elements("a[href]")
	if err != nil {
		return nil, fmt.Errorf("get links: %w", err)
	}

	var out []*url.URL
	for _, link := range links {
		href, err := link.Attribute("href")
		if err != nil {
			f.log.Warn("Failed to get href attribute of link", "err", err)
			continue
		}

		to, err := url.Parse(deref(href))
		if err != nil {
			f.log.Warn("Failed to parse link URL", "href", deref(href), "err", err)
			continue
		}

		if to.Host != f.rootURL.Host {
			continue
		}

		if visited.has(to.Path) || (f.pageLimit > 0 && visited.count() >= f.pageLimit) {
			continue
		}
		visited.add(to.Path)

		out = append(out, to)
	}

	return out, nil
}

func (f *Finder) extractClasses(page *rod.Page, pageUrl string) ([]usedClass, error) {
	found := make(map[string]int)

	elements, err := page.Elements("[class]")
	if err != nil {
		return nil, fmt.Errorf("get elements with class attribute: %w", err)
	}

	for _, el := range elements {
		rawClass, err := el.Attribute("class")
		if err != nil {
			f.log.Warn("Failed to get class attribute of element", "url", pageUrl, "err", err)
			continue
		}

		classList := strings.Split(deref(rawClass), " ")
		classList = filter(classList, func(s string) bool { return strings.TrimSpace(s) != "" })

		for _, class := range classList {
			found[class]++
		}
	}

	var out []usedClass
	for class, count := range found {
		out = append(out, usedClass{
			class: class,
			count: count,
		})
	}

	return out, nil
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

func deref[V any](v *V) V {
	if v == nil || v == (*V)(nil) {
		var zero V
		return zero
	}
	return *v
}

type visitedPages struct {
	sync.RWMutex
	paths map[string]bool
}

func (vp *visitedPages) add(path string) {
	vp.Lock()
	defer vp.Unlock()
	vp.paths[path] = true
}

func (vp *visitedPages) has(path string) bool {
	vp.RLock()
	defer vp.RUnlock()
	return vp.paths[path]
}

func (vp *visitedPages) count() int {
	vp.RLock()
	defer vp.RUnlock()
	return len(vp.paths)
}
