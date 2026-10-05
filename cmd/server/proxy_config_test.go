package main

import (
	"reflect"
	"testing"
)

func TestProxyConfig(t *testing.T) {
	cases := []struct {
		raw  string
		want []string
	}{
		{"", nil},
		{"  ,  ", nil},
		{"10.0.0.0/8", []string{"10.0.0.0/8"}},
		{" 10.0.0.0/8 , 172.16.0.1,,192.168.0.0/16 ", []string{"10.0.0.0/8", "172.16.0.1", "192.168.0.0/16"}},
	}
	for _, tc := range cases {
		if got := proxyConfig(tc.raw); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("proxyConfig(%q) = %#v, want %#v", tc.raw, got, tc.want)
		}
	}
}
