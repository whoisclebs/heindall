package main

import (
	"flag"
	"log"

	"github.com/whoisclebs/heindall/apps/api/internal/fraud"
)

func main() {
	referencesPath := flag.String("references", "resources/references.json.gz", "path to references.json.gz or JSON excerpt")
	outPath := flag.String("out", "resources/index.heindall.ivf8192.bin", "output binary index path")
	clusters := flag.Int("clusters", 8192, "IVF cluster count; must be a power of two")
	nprobe := flag.Int("nprobe", 8, "default IVF nprobe stored in the index")
	ambiguousNProbe := flag.Int("ambiguous-nprobe", 32, "default IVF nprobe for ambiguous results stored in the index")
	repair := flag.Bool("repair", true, "enable IVF bbox repair for ambiguous results")
	flag.Parse()

	refs, err := fraud.LoadReferences(*referencesPath)
	if err != nil {
		log.Fatalf("load references: %v", err)
	}
	if err := fraud.WriteIVFBinaryIndex(*outPath, refs, fraud.IVFBuildOptions{Clusters: *clusters, NProbe: *nprobe, AmbiguousNProbe: *ambiguousNProbe, Repair: *repair}); err != nil {
		log.Fatalf("write IVF index: %v", err)
	}
	log.Printf("wrote %d vectors to %s (ivf)", len(refs), *outPath)
}
