package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var urlRegex = regexp.MustCompile(`https?://[^\s\]\)>"']+`)

func main() {
	exts := flag.String("ext", ".md,.txt,.html", "Comma-separated list of file extensions")
	token := flag.String("token", "", "Bearer token for HTTP Authorization (optional)")
	verbose := flag.Bool("v", false, "Verbose output: print each URL and status")
	flag.Parse()

	if flag.NArg() < 1 {
		fmt.Println("❗ Usage: go run main.go DIRECTORY [--ext=.md,.txt,.html] [--token=YOUR_TOKEN] [-v]")
		os.Exit(1)
	}
	dir := flag.Arg(0)
	extensions := strings.Split(*exts, ",")

	files := []string{}
	filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		for _, ext := range extensions {
			if strings.HasSuffix(path, ext) {
				files = append(files, path)
				break
			}
		}
		return nil
	})

	checkedLinks := make(map[string]bool)
	brokenLinks := map[string][]string{}

	for _, file := range files {
		links := extractLinks(file)
		for _, link := range links {
			if result, exists := checkedLinks[link]; exists {
				if *verbose {
					status := "OK"
					if !result {
						status = "BROKEN"
					}
					fmt.Printf("🔁 Cached:  %s → %s (from cache)\n", link, status)
				}
				if !result {
					brokenLinks[file] = append(brokenLinks[file], link)
				}
				continue
			}
			ok, code := checkLink(link, *token)
			checkedLinks[link] = ok
			if *verbose {
				status := "OK"
				if !ok {
					status = "BROKEN"
				}
				fmt.Printf("🔎 Checking: %s → %d (%s)\n", link, code, status)
			}
			if !ok {
				brokenLinks[file] = append(brokenLinks[file], link)
			}
		}
	}

	if len(brokenLinks) == 0 {
		fmt.Println("✅ No broken links found.")
	} else {
		fmt.Println("\n🔍 Broken links:")
		for file, links := range brokenLinks {
			fmt.Println("📄", file)
			for _, l := range links {
				fmt.Println("   ❌", l)
			}
		}
	}
}

func extractLinks(file string) []string {
	f, err := os.Open(file)
	if err != nil {
		return nil
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	links := []string{}
	for scanner.Scan() {
		line := scanner.Text()
		found := urlRegex.FindAllString(line, -1)
		links = append(links, found...)
	}
	return links
}

func checkLink(url string, token string) (bool, int) {
	client := http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return false, 0
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		return false, 0
	}
	defer resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 300, resp.StatusCode
}
