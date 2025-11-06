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
	"log/slog"
	"net"
	"net/http"
	"os"
	"slices"
	"text/template"
	"time"
)

//go:generate sh -c "go run . > ../../realip.gen.go"

const cloudflareURI = `https://www.cloudflare.com/ips-%s`

//go:embed realip_cloudflare.tmpl
var cloudflareTmpl string

func writeTemplatedGoFile(f *os.File, tmpl *template.Template, data any) error {
	buf := &bytes.Buffer{}

	err := tmpl.Execute(buf, data)
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
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	var cidrs []*net.IPNet

	ctx, cancelFn := context.WithDeadline(context.Background(), time.Now().Add(time.Second*10))
	defer cancelFn()

	for _, version := range []string{"v4", "v6"} {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf(cloudflareURI, version), http.NoBody)
		if err != nil {
			logger.Error("error creating request", "error", err)
			os.Exit(1) //nolint:gocritic
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			logger.Error("error sending request", "error", err)
			os.Exit(1) //nolint:gocritic
		}

		if resp.StatusCode != http.StatusOK {
			_ = resp.Body.Close()
			logger.Error("unexpected status code", "status", resp.StatusCode)
			os.Exit(1) //nolint:gocritic
		}

		scan := bufio.NewScanner(resp.Body)

		var cidr *net.IPNet

		for scan.Scan() {
			_, cidr, err = net.ParseCIDR(scan.Text())
			if err != nil {
				_ = resp.Body.Close()
				logger.Error("error parsing CIDR", "error", err)
				os.Exit(1) //nolint:gocritic
			}

			// Make sure it doesn't already exist in the list.
			if slices.ContainsFunc(cidrs, func(other *net.IPNet) bool {
				return cidr.String() == other.String()
			}) {
				continue
			}

			cidrs = append(cidrs, cidr)
			logger.Info("found CIDR", "cidr", cidr)
		}
		_ = resp.Body.Close()
	}

	if len(cidrs) < 10 {
		logger.Error("found less than 10 CIDRs", "count", len(cidrs))
		os.Exit(1) //nolint:gocritic
	}

	logger.Info("found CIDRs", "count", len(cidrs))

	err := writeTemplatedGoFile(
		os.Stdout,
		template.Must(template.New(".").Parse(cloudflareTmpl)),
		cidrs,
	)
	if err != nil {
		logger.Error("error writing templated Go file", "error", err)
		os.Exit(1) //nolint:gocritic
	}
}
