package main

import (
	"log"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"

	"reel-fact-checker-go-temporal/factcheck"
	"reel-fact-checker-go-temporal/internal/config"
)

func main() {
	c, err := client.Dial(client.Options{HostPort: config.Address(), Namespace: config.Namespace()})
	if err != nil {
		log.Fatalln("unable to create client:", err)
	}
	defer c.Close()

	var extractor factcheck.ClaimExtractor = factcheck.MockExtractor{}
	var fetcher factcheck.AudioFetcher = factcheck.MockFetcher{}
	var transcriber factcheck.Transcriber = factcheck.MockTranscriber{}
	var searcher factcheck.Searcher = factcheck.MockSearcher{}
	var judge factcheck.ClaimJudge = factcheck.MockJudge{}
	var summarizer factcheck.Summarizer = factcheck.MockSummarizer{}

	w := worker.New(c, config.TaskQueue(), worker.Options{})
	w.RegisterWorkflow(factcheck.FactCheckWorkflow)
	w.RegisterActivity(&factcheck.Activities{
		Fetcher: fetcher, Transcriber: transcriber,
		Extractor: extractor, Searcher: searcher,
		Judge: judge, Summarizer: summarizer,
	})

	if err := w.Run(worker.InterruptCh()); err != nil {
		log.Fatalln("worker stopped:", err)
	}
}
