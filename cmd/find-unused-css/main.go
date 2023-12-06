package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"

	"github.com/bounoable/siteperf"
	"github.com/bounoable/siteperf/internal/plog"
)

var (
	rootURLRaw     = flag.String("url", "https://google.com", "Root URL to crawl")
	cssFilePathRaw = flag.String("css", "style.css", "Path to CSS file")
	limit          = flag.Int("limit", 0, "Limit the number of pages to visit")
	out            = flag.String("out", "", "Path to output file")
)

func main() {
	flag.Parse()

	defer plog.Debug()()

	if !strings.HasPrefix(*rootURLRaw, "https://") {
		*rootURLRaw = strings.TrimPrefix(*rootURLRaw, "https://")
		*rootURLRaw = strings.TrimPrefix(*rootURLRaw, "http://")
		*rootURLRaw = "https://" + *rootURLRaw
	}

	f, err := siteperf.New(*rootURLRaw, *limit)
	if err != nil {
		panic(err)
	}

	classes, err := siteperf.ExtractClassesFromFile(*cssFilePathRaw)
	if err != nil {
		panic(fmt.Errorf("extract classes from %q: %w", *cssFilePathRaw, err))
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	unused, err := f.FindUnused(ctx, classes)
	if err != nil {
		panic(err)
	}

	if *out != "" {
		if err := writeOutfile(unused); err != nil {
			panic(err)
		}
		fmt.Println("Wrote unused classes to", *out)
		return
	}

	out, err := json.MarshalIndent(unused, "", "  ")
	if err != nil {
		panic(err)
	}

	fmt.Println("Unused classes:")
	fmt.Println(string(out))
}

func writeOutfile(unused []string) error {
	path, err := filepath.Abs(*out)
	if err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	for _, name := range unused {
		if _, err := f.WriteString("." + name + "\n"); err != nil {
			return err
		}
	}

	return f.Close()
}
