package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"sync"
	"sync/atomic"

	"ra3d/torrent"
	"ra3d/tui"
)

func main() {
	outDir := flag.String("o", ".", "directory to download into")
	headless := flag.Bool("d", false, "no TUI, just log when each download ends (for running in the background)")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: ra3d [-o dir] [-d] [file.torrent | magnet-link]...\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if !*headless {
		if err := tui.Run(flag.Args(), *outDir); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	if flag.NArg() == 0 {
		flag.Usage()
		os.Exit(2)
	}
	var wg sync.WaitGroup
	var failed atomic.Bool
	for _, src := range flag.Args() {
		wg.Add(1)
		go func() {
			defer wg.Done()
			t := &torrent.Torrent{}
			if err := t.Run(src, *outDir); err != nil {
				log.Printf("%s: %v", src, err)
				failed.Store(true)
			} else {
				log.Printf("%s: done", t.Stats().Name)
			}
		}()
	}
	wg.Wait()
	if failed.Load() {
		os.Exit(1)
	}
}
