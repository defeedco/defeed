package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const defaultClientTimeout = 5 * time.Second

var DefaultHTTPClient = &http.Client{
	Transport: &http.Transport{
		MaxIdleConnsPerHost: 10,
	},
	Timeout: defaultClientTimeout,
}

var BuildVersion = "dev"

var PulseUserAgentString = "Pulse/" + BuildVersion + " +https://github.com/bartolomej/pulse"

type requestDoer interface {
	Do(*http.Request) (*http.Response, error)
}

func DecodeJSONFromRequest[T any](client requestDoer, request *http.Request) (T, error) {
	var result T

	response, err := client.Do(request)
	if err != nil {
		return result, err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return result, err
	}

	if response.StatusCode != http.StatusOK {
		truncatedBody, _ := LimitStringLength(string(body), 256)

		return result, fmt.Errorf(
			"unexpected status code %d from %s, response: %s",
			response.StatusCode,
			request.URL,
			truncatedBody,
		)
	}

	err = json.Unmarshal(body, &result)
	if err != nil {
		return result, err
	}

	return result, nil
}
