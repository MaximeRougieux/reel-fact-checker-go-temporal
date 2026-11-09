package main

import (
	"log"
	"os"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"

	"reel-fact-checker-go-temporal/factcheck"
	"reel-fact-checker-go-temporal/internal/config"
	"reel-fact-checker-go-temporal/internal/llmconfig"
)

func main() {
	config.LoadDotEnv(".env")

	var extractor factcheck.ClaimExtractor = factcheck.MockExtractor{}
	var judge factcheck.ClaimJudge = factcheck.MockJudge{}
	var summarizer factcheck.Summarizer = factcheck.MockSummarizer{}
	if provider := llmconfig.Provider(); provider != "" {
		if os.Getenv(llmconfig.KeyVar(provider)) == "" {
			log.Fatalf("LLM provider %q needs %s", provider, llmconfig.KeyVar(provider))
		}
		extractLLM, err := llmconfig.NewLLM(provider, llmconfig.ExtractorModel())
		if err != nil {
			log.Fatalln(err)
		}
		judgeLLM, err := llmconfig.NewLLM(provider, llmconfig.JudgeModel())
		if err != nil {
			log.Fatalln(err)
		}
		if extractor, err = factcheck.NewLLMExtractor(extractLLM); err != nil {
			log.Fatalln(err)
		}
		if judge, err = factcheck.NewLLMJudge(judgeLLM); err != nil {
			log.Fatalln(err)
		}
		summarizer = factcheck.NewLLMSummarizer(judgeLLM)
		log.Println("LLM provider:", provider, "| extractor model:", llmconfig.ExtractorModel(), "| judge and summarizer model:", llmconfig.JudgeModel())
	} else {
		log.Println("claim extractor, judge, summarizer: mock (no ANTHROPIC_API_KEY or MISTRAL_API_KEY)")
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
	switch config.Transcriber() {
	case "whisper":
		transcriber = &factcheck.WhisperTranscriber{CLI: config.WhisperCLI(), Model: config.WhisperModel(), AudioDir: config.AudioDir()}
		log.Println("transcriber: local whisper.cpp")
	case "voxtral":
		if os.Getenv("MISTRAL_API_KEY") == "" {
			log.Fatalln("TRANSCRIBER=voxtral needs MISTRAL_API_KEY")
		}
		transcriber = factcheck.NewVoxtralTranscriber(config.VoxtralModel(), config.AudioDir())
		log.Println("transcriber: Mistral Voxtral", config.VoxtralModel())
	default:
		log.Println("transcriber: mock")
	}

	var searcher factcheck.Searcher = factcheck.MockSearcher{}
	if os.Getenv("TAVILY_API_KEY") != "" {
		searcher = factcheck.NewTavilySearcher(config.SearchTopic(), config.SearchDepth())
		log.Println("searcher: Tavily")
	} else {
		log.Println("searcher: mock (no TAVILY_API_KEY)")
	}

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
