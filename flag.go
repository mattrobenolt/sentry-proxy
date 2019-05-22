package main

import (
	"flag"
	"fmt"
	"net/url"
)

type urlValue url.URL

func newUrlValue(val *url.URL, p *url.URL) *urlValue {
	*p = *val
	return (*urlValue)(p)
}

func (f *urlValue) Set(value string) error {
	upstream, err := url.Parse(value)
	if err != nil {
		return err
	}
	if upstream.Scheme == "" || upstream.Host == "" {
		return fmt.Errorf("invalid url")
	}
	upstream.Path = ""
	*f = urlValue(*upstream)
	return nil
}

func (f *urlValue) String() string {
	return (*url.URL)(f).String()
}

func UpstreamFlag(name string, value *url.URL, usage string) *url.URL {
	p := new(url.URL)
	flag.Var(newUrlValue(value, p), name, usage)
	return p
}
