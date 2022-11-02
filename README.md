<h1 align="center">
  <img src="https://user-images.githubusercontent.com/8293321/196779266-421c79d4-643a-4f73-9b54-3da379bbac09.png" alt="katana" width="200px">
  <br>
</h1>

<h4 align="center">A next-generation crawling and spidering framework</h4>

<p align="center">
<a href="https://goreportcard.com/report/github.com/projectdiscovery/katana"><img src="https://goreportcard.com/badge/github.com/projectdiscovery/katana"></a>
<a href="https://github.com/projectdiscovery/katana/issues"><img src="https://img.shields.io/badge/contributions-welcome-brightgreen.svg?style=flat"></a>
<a href="https://github.com/projectdiscovery/katana/releases"><img src="https://img.shields.io/github/release/projectdiscovery/katana"></a>
<a href="https://twitter.com/pdiscoveryio"><img src="https://img.shields.io/twitter/follow/pdiscoveryio.svg?logo=twitter"></a>
<a href="https://discord.gg/projectdiscovery"><img src="https://img.shields.io/discord/695645237418131507.svg?logo=discord"></a>
</p>

<p align="center">
  <a href="#features">Features</a> •
  <a href="#installation">Installation</a> •
  <a href="#usage">Usage</a> •
  <a href="#running-katana">Running katana</a> •
  <a href="https://discord.gg/projectdiscovery">Join Discord</a>
</p>


# Features

![image](https://user-images.githubusercontent.com/8293321/199371558-daba03b6-bf9c-4883-8506-76497c6c3a44.png)

 - Fast And fully configurable web crawling
 - **Standard** and **Headless** mode support
 - **JavaScript** parsing / crawling
 - Customizable **automatic form filling**
 - Pre-Configured / Regex based **Scope control**
 - Pre-Configured fields for **customizable output**
 - **URL** / **LIST** input support
 - STD **IN/OUT** and **TXT/JSON** output


## Installation

katana requires **Go 1.18** to install successfully. To install, just run the below command or download pre-compiled binary from [release page](https://github.com/projectdiscovery/katana/releases).

```console
go install github.com/projectdiscovery/katana/cmd/katana@latest
```

## Usage

```console
katana -h
```

This will display help for the tool. Here are all the switches it supports.

```console
Usage:
  ./katana [flags]

Flags:
INPUT:
   -u, -list string[]  target url / list to crawl

CONFIGURATIONS:
   -config string                path to the nuclei configuration file
   -d, -depth int                maximum depth to crawl (default 2)
   -kf, -known-files string      enable crawling of known files (all,robotstxt,sitemapxml)
   -ct, -crawl-duration int      maximum duration to crawl the target for
   -mrs, -max-response-size int  maximum response size to read (default 2097152)
   -timeout int                  time to wait for request in seconds (default 10)
   -retries int                  number of times to retry the request (default 1)
   -proxy string                 http/socks5 proxy to use
   -he, -headless                enable experimental headless hybrid crawling (process in one pass raw http requests/responses and dom-javascript web pages in browser context)
   -form-config string           path to custom form configuration file
   -H, -headers string[]         custom header/cookie to include in request
   -sc, -system-chrome           Use local installed chrome browser instead of nuclei installed
   -sb, -show-browser            show the browser on the screen with headless mode

FILTERS:
   -fs, -field-scope string         pre-defined scope field (dn,rdn,fqdn) (default "rdn")
   -ns, -no-scope                   disables host based default scope
   -cs, -crawl-scope string[]       in scope url regex to be followed by crawler
   -cos, -crawl-out-scope string[]  out of scope url regex to be excluded by crawler
   -jc, -js-crawl                   enable endpoint parsing / crawling in javascript file
   -e, -extension string[]          extensions to be explicitly allowed for crawling (* means all - default)
   -extensions-allow-list string[]  extensions to allow from default deny list
   -extensions-deny-list string[]   custom extensions for the crawl extensions deny list

RATE-LIMIT:
   -c, -concurrency int          number of concurrent fetchers to use (default 10)
   -p, -parallelism int          number of concurrent inputs to process (default 10)
   -rd, -delay int               request delay between each request in seconds
   -rl, -rate-limit int          maximum requests to send per second (default 150)
   -rlm, -rate-limit-minute int  maximum number of requests to send per minute

OUTPUT:
   -o, -output string         file to write output to
   -f, -fields string         field to display in output (url,path,fqdn,rdn,rurl,qurl,file,key,value,kv,dir,udir)
   -sf, -store-fields string  field to store in per-host output (url,path,fqdn,rdn,rurl,qurl,file,key,value,kv,dir,udir)
   -j, -json                  write output in JSONL(ines) format
   -nc, -no-color             disable output content coloring (ANSI escape codes)
   -silent                    display output only
   -v, -verbose               display verbose output
   -version                   display project version
```

## Running Katana

### Input for katana

**katana** requires **url** or **endpoint** to crawl and accepts single or multiple inputs.

Input URL can be provided using `-u` option, and multiple values can be provided using comma-separated input, similarly **file** input is supported using `-list` option and additionally piped input (stdin) is also supported.


```sh
katana -u https://tesla.com # Single URL input
```

```sh
katana -u https://tesla.com,https://google.com # Multiple URL input (comma-separated)
```

```sh
katana -list url_list.txt # List input
```

```sh
echo https://tesla.com | katana # STDIN (piped) input
```

```sh
cat domains | httpx | katana # STDIN (piped) input
```


## Crawl Mode

### Standard Mode

### Headless Mode

----

### Katana Field

### Rate & Delay

### Scope Control

## Crawl Config