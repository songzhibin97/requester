package requester

import (
	"context"
	"encoding/json"
	"errors"
	"html/template"
	"net/http"
	"regexp"
	"strings"

	"github.com/go-resty/resty/v2"
	sprig "github.com/go-task/slim-sprig/v3"
	"github.com/moul/http2curl"
	"github.com/songzhibin97/go-baseutils/base/banytostring"
	"github.com/songzhibin97/go-baseutils/base/breflect"
	"github.com/songzhibin97/go-ognl"
)

type Method string

var (
	GET   Method = "GET"
	POST  Method = "POST"
	PUT   Method = "PUT"
	DEL   Method = "DELETE"
	PATCH Method = "PATCH"
)

type Requester struct {
	URL    string `json:"url"`
	Method Method `json:"method"`

	Headers   map[string]string `json:"headers"`
	Params    map[string]string `json:"params"`
	Body      string            `json:"body"`
	BodyParam map[string]string `json:"bodyParam"`

	ParseResponseValue map[string]string `json:"parseResponseValue"`
}

func NewRequester(url string, method Method, header, param map[string]string, body string, bodyParam map[string]string, parseResponseValue map[string]string) *Requester {
	return &Requester{
		URL:                url,
		Method:             method,
		Headers:            header,
		Params:             param,
		Body:               body,
		BodyParam:          bodyParam,
		ParseResponseValue: parseResponseValue,
	}
}

// Request
// debug is true return curl
// return response, curl , err
func (r Requester) Request(ctx context.Context, client *resty.Client, debug bool) (any, string, error) {
	return request[map[string]interface{}](ctx, client, debug, r.URL, r.Method, r.Headers, r.Params, r.Body, r.BodyParam)
}

// ParseResponse
// 解析响应
func (r Requester) ParseResponse(response any) map[string]string {
	parse := make(map[string]string)
	for k, v := range r.ParseResponseValue {
		vv := ognl.Get(response, v).Value()
		if !breflect.IsNil(vv) {
			parse[k] = banytostring.ToString(vv)
		} else {
			parse[k] = ""
		}
	}

	return parse
}

func request[Response any](ctx context.Context, client *resty.Client, debug bool, url string, method Method, headers map[string]string, payload map[string]string, body string, bodyParam map[string]string) (Response, string, error) {
	var curl string
	query := client.SetPreRequestHook(func(client *resty.Client, h *http.Request) error {
		if debug {
			resp, err := http2curl.GetCurlCommand(h)
			if err != nil {
				return err
			}
			curl = resp.String()
		}
		return nil
	}).
		R().SetContext(ctx).
		SetHeader("Content-Type", "application/json")

	for k, v := range headers {
		query = query.SetHeader(k, v)
	}

	for k, v := range payload {
		query = query.SetQueryParam(k, v)
	}

	var zeroResponse Response

	if len(body) != 0 {
		// 判断bodyParam是否为空,不为空则使用模版替换
		if len(bodyParam) != 0 {
			builder := strings.Builder{}
			t, err := template.New("temp").Funcs(sprig.FuncMap()).Parse(string(body))
			if err != nil {
				return zeroResponse, curl, err
			}
			err = t.Execute(&builder, bodyParam)
			if err != nil {
				return zeroResponse, curl, err
			}
			body = builder.String()
		}

		query = query.SetBody(body)
	}

	var (
		response *resty.Response
		err      error
	)

	switch method {
	case GET:
		response, err = query.Get(url)
	case POST:
		response, err = query.Post(url)
	case PUT:
		response, err = query.Put(url)
	case DEL:
		response, err = query.Delete(url)
	case PATCH:
		response, err = query.Patch(url)
	default:
		return zeroResponse, "", errors.New("unsupported method")
	}

	if err != nil {
		return zeroResponse, curl, err
	}

	if response.StatusCode() != 200 {
		return zeroResponse, curl, errors.New(response.String())
	}

	err = json.Unmarshal(response.Body(), &zeroResponse)
	if err != nil {
		return zeroResponse, curl, err
	}
	return zeroResponse, curl, nil
}

// ReplacedArg 替换路径参数
// 使用正则替换表达式参数
func ReplacedArg(args any, val string, regular string, escape bool) string {
	re := regexp.MustCompile(regular)
	return re.ReplaceAllStringFunc(val, func(placeholder string) string {
		// Extract the key from the placeholder
		matches := re.FindStringSubmatch(placeholder)
		if len(matches) > 1 {
			key := matches[1]

			if escape {
				key = strings.ReplaceAll(key, ".", "\\.")
			}

			result := ognl.Get(args, key).Value()
			if !breflect.IsNil(result) {
				return banytostring.ToString(result)
			}
		}
		// If key not found in data map, return the placeholder as is
		return placeholder
	})
}

// SearchReplacedArg 查找替换参数
func SearchReplacedArg(val string, regular string) []string {
	var res []string

	re := regexp.MustCompile(regular)
	re.ReplaceAllStringFunc(val, func(placeholder string) string {
		// Extract the key from the placeholder
		matches := re.FindStringSubmatch(placeholder)
		if len(matches) > 1 {
			key := matches[1]
			res = append(res, key)
			return key

		}
		// If key not found in data map, return the placeholder as is
		return placeholder
	})

	return res
}
