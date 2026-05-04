package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/whoisclebs/heindall/apps/api/internal/fraud"
)

type testDataFile struct {
	Stats   json.RawMessage `json:"stats"`
	Entries []testEntry     `json:"entries"`
}

type testEntry struct {
	Request          json.RawMessage `json:"request"`
	ExpectedApproved bool            `json:"expected_approved"`
}

func main() {
	indexPath := flag.String("index", "resources/index.heindall.ivf8192.bin", "path to binary index")
	testDataPath := flag.String("test-data", "../../specs/test/test-data.json", "path to official test-data.json")
	nprobe := flag.Int("nprobe", 8, "IVF base nprobe")
	ambiguousNProbe := flag.Int("ambiguous-nprobe", 32, "IVF ambiguous nprobe")
	repair := flag.Bool("repair", true, "enable IVF bbox repair")
	limit := flag.Int("limit", 0, "optional max entries to evaluate")
	errorsOut := flag.String("errors-out", "", "optional path to write misclassified entries")
	only := flag.String("only", "", "optional comma-separated test entry indices to evaluate")
	flag.Parse()

	idx, err := fraud.LoadBinaryIndex(*indexPath)
	if err != nil {
		log.Fatalf("load index: %v", err)
	}
	idx.SetIVFSearch(*nprobe, *ambiguousNProbe, *repair)

	data, err := loadTestData(*testDataPath)
	if err != nil {
		log.Fatalf("load test data: %v", err)
	}
	entries := data.Entries
	if *limit > 0 && *limit < len(entries) {
		entries = entries[:*limit]
	}
	onlySet := parseOnlySet(*only)

	norm := fraud.DefaultNormalization()
	durations := make([]time.Duration, 0, len(entries))
	var tp, tn, fp, fn, invalid int
	errors := make([]map[string]any, 0)
	for i, entry := range entries {
		if onlySet != nil && !onlySet[i] {
			continue
		}
		start := time.Now()
		query, ok := fraud.VectorizeJSONQuantized(entry.Request, norm)
		if !ok {
			invalid++
			continue
		}
		frauds := idx.Search5Quantized(query)
		durations = append(durations, time.Since(start))
		approved := frauds < 3
		if approved == entry.ExpectedApproved {
			if approved {
				tn++
			} else {
				tp++
			}
		} else if approved {
			fn++
			errors = append(errors, errorEntry(i, entry, frauds, approved))
		} else {
			fp++
			errors = append(errors, errorEntry(i, entry, frauds, approved))
		}
	}
	if *errorsOut != "" {
		if err := writeJSON(*errorsOut, errors); err != nil {
			log.Fatalf("write errors: %v", err)
		}
	}

	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	total := tp + tn + fp + fn + invalid
	weightedErrors := fp + fn*3 + invalid*5
	failureRate := 0.0
	if total > 0 {
		failureRate = float64(fp+fn+invalid) / float64(total)
	}

	result := map[string]any{
		"total":             total,
		"true_positive":     tp,
		"true_negative":     tn,
		"false_positive":    fp,
		"false_negative":    fn,
		"invalid":           invalid,
		"weighted_errors_E": weightedErrors,
		"failure_rate":      failureRate,
		"p50_ms":            millis(percentile(durations, 0.50)),
		"p95_ms":            millis(percentile(durations, 0.95)),
		"p99_ms":            millis(percentile(durations, 0.99)),
		"nprobe":            *nprobe,
		"ambiguous_nprobe":  *ambiguousNProbe,
		"repair":            *repair,
	}
	out, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(out))
}

func parseOnlySet(value string) map[int]bool {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	out := make(map[int]bool)
	for _, part := range strings.Split(value, ",") {
		idx, err := strconv.Atoi(strings.TrimSpace(part))
		if err == nil {
			out[idx] = true
		}
	}
	return out
}

func errorEntry(index int, entry testEntry, frauds int, approved bool) map[string]any {
	var req map[string]any
	_ = json.Unmarshal(entry.Request, &req)
	return map[string]any{
		"index":             index,
		"frauds":            frauds,
		"approved":          approved,
		"expected_approved": entry.ExpectedApproved,
		"request":           req,
	}
}

func writeJSON(path string, value any) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(value)
}

func loadTestData(path string) (testDataFile, error) {
	f, err := os.Open(path)
	if err != nil {
		return testDataFile{}, err
	}
	defer f.Close()
	var data testDataFile
	if err := json.NewDecoder(f).Decode(&data); err != nil {
		return testDataFile{}, err
	}
	return data, nil
}

func percentile(values []time.Duration, p float64) time.Duration {
	if len(values) == 0 {
		return 0
	}
	idx := int(float64(len(values)-1) * p)
	return values[idx]
}

func millis(value time.Duration) float64 {
	return float64(value.Nanoseconds()) / 1_000_000.0
}
