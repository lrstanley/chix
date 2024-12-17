// Copyright (c) Liam Stanley <liam@liam.sh>. All rights reserved. Use of
// this source code is governed by the MIT license that can be found in
// the LICENSE file.

package main

import (
	"bufio"
	"bytes"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"go/format"
	"net"
	"net/http"
	"os"
	"slices"
	"text/template"
	"time"
)

const cloudflareURI = `https://www.cloudflare.com/ips-%s`

//go:embed realip_cloudflare.tmpl
var cloudflareTmpl string

func writeTemplatedGoFile(path string, tmpl *template.Template, data any) error {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return errors.New("error opening file: " + err.Error())
	}
	defer f.Close()

	buf := &bytes.Buffer{}

	err = tmpl.Execute(buf, data)
	if err != nil {
		return errors.New("error executing template: " + err.Error())
	}

	fmtd, err := format.Source(buf.Bytes())
	if err != nil {
		return errors.New("error formatting source: " + err.Error())
	}

	_, err = f.Write(fmtd)
	return err
}

func main() {
	var cidrs []*net.IPNet

	ctx, cancelFn := context.WithDeadline(context.Background(), time.Now().Add(time.Second*10))
	defer cancelFn()

	for _, version := range []string{"v4", "v6"} {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf(cloudflareURI, version), http.NoBody)
		if err != nil {
			panic(err)
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			panic(err)
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			panic(fmt.Errorf("unexpected status code: %d", resp.StatusCode))
		}

		scan := bufio.NewScanner(resp.Body)

		var cidr *net.IPNet

		for scan.Scan() {
			_, cidr, err = net.ParseCIDR(scan.Text())
			if err != nil {
				resp.Body.Close()
				panic(err)
			}

			// Make sure it doesn't already exist in the list.
			if slices.ContainsFunc(cidrs, func(other *net.IPNet) bool {
				return cidr.String() == other.String()
			}) {
				continue
			}

			cidrs = append(cidrs, cidr)
			fmt.Printf("found CIDR: %s\n", cidr) //nolint:forbidigo
		}
		_ = resp.Body.Close()
	}

	if len(cidrs) < 10 {
		panic(fmt.Errorf("found %d CIDRs, but expected at least 10", len(cidrs)))
	}

	fmt.Printf("found %d CIDRs\n", len(cidrs)) //nolint:forbidigo

	err := writeTemplatedGoFile(
		"realip_cloudflare.go",
		template.Must(template.New(".").Parse(cloudflareTmpl)),
		cidrs,
	)
	if err != nil {
		panic(err)
	}
}
