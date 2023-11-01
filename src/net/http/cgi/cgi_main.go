package cgi

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

func main() {
	req, err := Request()
	if err != nil {
		panic(err)
	}
	query := req.URL.Query()
	if query.Get("loc") != "" {
		fmt.Printf("Location: %s\r\n\r\n", query.Get("loc"))
		return
	}

	err = req.ParseForm()
	if err != nil {
		panic(err)
	}

	for s, i := range req.Form {
		query.Set(s, i[0])
	}

	fmt.Printf("Content-Type: text/html\r\n")
	fmt.Printf("X-CGI-Pid: %d\r\n", os.Getpid())
	fmt.Printf("X-Test-Header: X-Test-Value\r\n")
	fmt.Printf("\r\n")

	if query.Get("writestderr") != "" {
		fmt.Fprintf(os.Stderr, "Hello, stderr!\n")
	}

	if query.Get("bigresponse") != "" {
		// 17 MB, for OS X: golang.org/issue/4958
		for i := 0; i < 17*1024; i++ {
			fmt.Printf("%s\r\n", strings.Repeat("A", 1024))
		}
		return
	}

	fmt.Printf("test=Hello CGI\r\n")

	keys := make([]string, 0, len(query))
	for k := range query {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, key := range keys {
		fmt.Printf("param-%s=%s\r\n", key, query.Get(key))
	}

	envs := envMap(os.Environ())
	keys = make([]string, 0, len(envs))
	for k := range envs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, key := range keys {
		fmt.Printf("env-%s=%s\r\n", key, envs[key])
	}

	cwd, _ := os.Getwd()
	fmt.Printf("cwd=%s\r\n", cwd)
}
