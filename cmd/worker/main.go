package main

import (
	"log"
	"os"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"

	"reel-fact-checker-go-temporal/factcheck"
	"reel-fact-checker-go-temporal/internal/config"
)

func main() {
	config.LoadDotEnv(".env")

	var extractor factcheck.ClaimExtractor = factcheck.MockExtractor{}
	if os.Getenv("ANTHROPIC_API_KEY") != "" {
		e, err := factcheck.NewClaudeExtractor(config.ExtractorModel())
		if err != nil {
			log.Fatalln(err)
		}
		extractor = e
		log.Println("claim extractor: Claude", config.ExtractorModel())
	} else {
		log.Println("claim extractor: mock (no ANTHROPIC_API_KEY)")
	}

	c, err := client.Dial(client.Options{HostPort: config.Address(), Namespace: config.Namespace()})
	if err != nil {
		log.Fatalln("unable to create client:", err)
	}
	defer c.Close()

	var fetcher factcheck.AudioFetcher = factcheck.MockFetcher{}
	if config.Fetcher() == "browser" {
		fetcher = &factcheck.BrowserFetcher{Dir: config.AudioDir(), FFmpeg: config.FFmpeg()}
		log.Println("audio fetcher: headless browser")
	} else {
		log.Println("audio fetcher: mock")
	}

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
