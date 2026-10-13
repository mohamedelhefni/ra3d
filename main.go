package main

import (
	"flag"
	"fmt"
	"os"

	"ra3d/torrent"
	"ra3d/tui"
)

func main() {
	outDir := flag.String("o", ".", "directory to download into")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: ra3d [-o dir] <file.torrent | magnet-link>\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}

	t := &torrent.Torrent{}
	go t.Run(flag.Arg(0), *outDir)
	if err := tui.Run(t); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if t.Stats().Err != nil {
		os.Exit(1)
	}
}
