package gon

import (
	"fmt"
	"log"
	"net/http"
)

// Postfix "!" propagates the error, so the response is valid afterwards.
func bangGet(url string) error {
	res := http.Get(url)!
	defer res.Body.Close()
	return nil
}

func bangParenthesized(url string) error {
	res := (http.Get(url))!
	defer res.Body.Close()
	return nil
}

func bangClientDo(client *http.Client, req *http.Request) error {
	resp := client.Do(req)!
	defer resp.Body.Close()
	return nil
}

func bangClientValueGet(url string) error {
	client := http.Client{}
	resp := client.Get(url)!
	defer resp.Body.Close()
	return nil
}

// An "or" handler of a call with success results must terminate,
// so the response is valid after it.
func orHandler(client *http.Client, req *http.Request) error {
	resp := client.Do(req) or err {
		return fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()
	return nil
}

func orHandlerPackageFunc(url string) {
	res := http.Get(url) or err {
		log.Fatal(err)
		return
	}
	defer res.Body.Close()
}

func orHandlerParenthesized(client *http.Client, req *http.Request) error {
	resp := (client.Do(req)) or err {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// The handled call may be an argument of another statement kind or
// nested in a handler: only the call directly handled is exempt.
func nestedHandlers(client *http.Client, req *http.Request) error {
	resp := client.Do(req) or err {
		retry, err2 := http.Get("http://foo.com")
		defer retry.Body.Close() // want "using retry before checking for errors"
		if err2 != nil {
			return err
		}
		return err
	}
	defer resp.Body.Close()
	return nil
}

func lambdaHandlers(url string) func() error {
	return () => {
		res := http.Get(url)!
		defer res.Body.Close()
		return nil
	}
}

// Existing Go diagnostics are retained next to Gon syntax.
func badHTTPGet() {
	res, err := http.Get("http://foo.com")
	defer res.Body.Close() // want "using res before checking for errors"
	if err != nil {
		log.Fatal(err)
	}
}

func badClientDo(client *http.Client, req *http.Request) {
	resp, err := client.Do(req)
	defer resp.Body.Close() // want "using resp before checking for errors"
	if err != nil {
		log.Fatal(err)
	}
}

func badAfterHandled(client *http.Client, req *http.Request) error {
	req2 := http.NewRequest("GET", "http://foo.com", nil)!
	resp, err := client.Do(req2)
	defer resp.Body.Close() // want "using resp before checking for errors"
	if err != nil {
		return err
	}
	return nil
}

func goodGoStyle(client *http.Client, req *http.Request) error {
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}
