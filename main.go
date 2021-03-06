package main

import (
	"bufio"
	pc "github.com/0xsha/ProxyFor/internal"
	"github.com/akamensky/argparse"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"os"
	"strconv"
	"strings"
	"sync"
)

func main() {

	// beautify logs
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	// parse arguments
	parser := argparse.NewParser("ProxyChecker", "Checks for valid proxies and write valid ones in file")
	concurrency := parser.Int("t", "threads",
		&argparse.Options{
			Required: false,
			Help:     "Number of threads",
			Default:  40,
		})

	responseCode := parser.Int("r", "response",
		&argparse.Options{
			Required: false,
			Help:     "expected HTTP response code",
			Default:  200,
		})

	path := parser.String("p", "path",
		&argparse.Options{
			Required: false,
			Help:     "path to proxy.txt (required)"})

	domain := parser.String("d", "domain",
		&argparse.Options{
			Required: false,
			Help:     "Domain to check proxies against it",
			Default:  "https://httpbin.org/ip"})

	output := parser.String("o", "output",
		&argparse.Options{
			Default:  "out.txt",
			Required: false,
			Help:     "Output file"})

	// capital T for timeouts
	timeout := parser.Int("T", "timeout",
		&argparse.Options{
			Required: false,
			Help:     "timeout in seconds",
			Default:  10})

	err := parser.Parse(os.Args)
	if err != nil {
		log.Fatal().Err(err).Msg(parser.Usage(err))
	}

	var proxyList []string
	// input as stdin
	stat, _ := os.Stdin.Stat()
	// detect if anything came from std
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		sc := bufio.NewScanner(os.Stdin)
		for sc.Scan() {
			raw := strings.TrimSpace(sc.Text())
			if err := sc.Err(); err == nil && raw != "" {
				proxyList = append(proxyList, raw)
			}
		}
	}

	if *path != "" {
		proxyInput, err := pc.ReadFile(*path)
		if len(proxyInput) > 0 || err == nil {
			proxyList = append(proxyList, proxyInput...)
		}
	}
	if err != nil && len(proxyList) == 0 {
		log.Fatal().Err(err).Msg("No input provided")
	}
	proxyList = pc.Unique(proxyList)
	//currentIP := pc.GetCurrentIP()

	//let's find valid http(s) proxies
	httpProxyListChannel := make(chan string, len(proxyList))
	validHTTPChannel := make(chan pc.ValidProxy)
	var validHTTP []pc.ValidProxy

	var hwg sync.WaitGroup

	log.Info().Msg("HTTP(s) hunt began")

	for i := 0; i < *concurrency; i++ {
		go pc.CheckHTTPProxy(httpProxyListChannel, &hwg, *timeout, validHTTPChannel, *domain, *responseCode)
	}

	go func() {
		for _, proxyLine := range proxyList {
			httpProxyListChannel <- proxyLine
		}
		close(httpProxyListChannel)
	}()

	go func() {
		for valid := range validHTTPChannel {
			log.Info().Msg(valid.Address)
			validHTTP = append(validHTTP, valid)
		}

	}()
	hwg.Wait()

	log.Info().Msg("Total HTTP(s) proxies found: " + strconv.Itoa(len(validHTTP)))

	// sort proxies by response time
	sortedHTTP := pc.SortByResponseTime(validHTTP)

	if len(sortedHTTP) > 0 {
		//outName := pc.GenerateOutputName("http.txt")
		// write them to file
		pc.WriteProxiesToFile(sortedHTTP, *output)
	} else {
		log.Warn().Msg("No valid HTTP(s) proxies found.")
	}

	// let's find valid Socks5 proxies
	validSOCKSChannel := make(chan pc.ValidProxy)
	socksProxyListChannel := make(chan string, len(proxyList))

	var validSocks []pc.ValidProxy

	var swg sync.WaitGroup

	log.Info().Msg("Socks5 hunt began")
	for i := 0; i < *concurrency; i++ {
		go pc.CheckSocks5Proxy(socksProxyListChannel, &swg, *timeout, validSOCKSChannel, *domain, *responseCode)
	}

	go func() {
		for _, proxyLine := range proxyList {
			socksProxyListChannel <- proxyLine
		}
		close(socksProxyListChannel)
	}()

	go func() {

		for valid := range validSOCKSChannel {

			//fmt.Println(valid.Anonymous , valid.ResponseTime)
			validSocks = append(validSocks, valid)

		}

	}()
	swg.Wait()

	// sort proxies by response time
	sortedSocks := pc.SortByResponseTime(validSocks)

	if len(sortedSocks) > 0 {

		// write them to file
		//outName := pc.GenerateOutputName("socks.txt")
		pc.WriteProxiesToFile(sortedSocks, *output)

		log.Info().Msg("Total Socks5 proxies found:" + strconv.Itoa(len(validSocks)))

	} else {

		log.Warn().Msg("No Valid Socks5 proxy found ")
	}

}
