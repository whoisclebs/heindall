package main

import (
	"flag"
	"log"

	"github.com/whoisclebs/heindall/apps/api/internal/fraud"
)

func main() {
	referencesPath := flag.String("references", "resources/references.json.gz", "path to references.json.gz or JSON excerpt")
	outPath := flag.String("out", "resources/index.heindall.bin", "output binary index path")
	flag.Parse()

	refs, err := fraud.LoadReferences(*referencesPath)
	if err != nil {
		log.Fatalf("load references: %v", err)
	}
	if err := fraud.WriteBinaryIndex(*outPath, refs); err != nil {
		log.Fatalf("write index: %v", err)
	}
	log.Printf("wrote %d vectors to %s", len(refs), *outPath)
}
