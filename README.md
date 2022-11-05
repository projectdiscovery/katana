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
 - **Scope control** - Preconfigured field / Regex 
 - **Customizable output** - Preconfigured fields
 - INPUT - **STDIN**, **URL** and **LIST**
 - OUTPUT - **STDOUT**, **FILE** and **JSON**


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

CONFIGURATION:
   -d, -depth int                maximum depth to crawl (default 2)
   -jc, -js-crawl                enable endpoint parsing / crawling in javascript file
   -ct, -crawl-duration int      maximum duration to crawl the target for
   -kf, -known-files string      enable crawling of known files (all,robotstxt,sitemapxml)
   -mrs, -max-response-size int  maximum response size to read (default 2097152)
   -timeout int                  time to wait for request in seconds (default 10)
   -retry int                    number of times to retry the request (default 1)
   -proxy string                 http/socks5 proxy to use
   -H, -headers string[]         custom header/cookie to include in request
   -config string                path to the nuclei configuration file
   -fc, -form-config string      path to custom form configuration file

HEADLESS:
   -he, -headless       enable experimental headless hybrid crawling (process in one pass raw http requests/responses and dom-javascript web pages in browser context)
   -sc, -system-chrome  Use local installed chrome browser instead of nuclei installed
   -sb, -show-browser   show the browser on the screen with headless mode

SCOPE:
   -cs, -crawl-scope string[]       in scope url regex to be followed by crawler
   -cos, -crawl-out-scope string[]  out of scope url regex to be excluded by crawler
   -fs, -field-scope string         pre-defined scope field (dn,rdn,fqdn) (default "rdn")
   -ns, -no-scope                   disables host based default scope
   -do, -display-out-scope          display external endpoint from scoped crawling

FILTER:
   -f, -field string               field to display in output (url,path,fqdn,rdn,rurl,qurl,qpath,file,key,value,kv,dir,udir)
   -sf, -store-field string        field to store in per-host output (url,path,fqdn,rdn,rurl,qurl,qpath,file,key,value,kv,dir,udir)
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
   -o, -output string  file to write output to
   -j, -json           write output in JSONL(ines) format
   -nc, -no-color      disable output content coloring (ANSI escape codes)
   -silent             display output only
   -v, -verbose        display verbose output
   -version            display project version
```

## Running Katana

### Input for katana

**katana** requires **url** or **endpoint** to crawl and accepts single or multiple inputs.

Input URL can be provided using `-u` option, and multiple values can be provided using comma-separated input, similarly **file** input is supported using `-list` option and additionally piped input (stdin) is also supported.

#### URL Input

```sh
katana -u https://tesla.com
```

#### Multiple URL Input (comma-separated)

```sh
katana -u https://tesla.com,https://google.com
```

#### List Input
```sh
cat url_list.txt

https://tesla.com
https://google.com
```

```
katana -list url_list.txt
```

#### STDIN (piped) Input

```sh
echo https://tesla.com | katana
```

```sh
cat domains | httpx | katana
```


## Crawling Mode

### Standard Mode

### Headless Mode

## Scope Control

Crawling can be endless if not scoped, as such katana comes with multiple support to define the crawl scope.

*field-scope*
----
Most handy option to define scope with predefined field name, `rdn` being default option for field scope.

   - `rdn` - crawling scoped to root domain name and all subdomains (default)
   - `fqdn` - crawling scoped to given sub(domain) 
   - `dn` - crawling scoped to domain name keyword

```
katana -u https://tesla.com -fs dn
```


*crawl-scope*
------

For advanced scope control, `-cs` option can be used that comes with **regex** support.

```
katana -u https://tesla.com -cs login
```


*crawl-out-scope*
-----

For defining what not to crawl, `-cos` option can be used and also support **regex** input.

```
katana -u https://tesla.com -cs logout
```

*no-scope*
----

Katana is default to scope `*.domain`, to disable this `-ns` option can be used and also to crawl the internet.

```
katana -u https://tesla.com -ns
```

*display-out-scope*
----

As default, when scope option is used, it also applies for the links to display as output, as such **external URLs are default to exclude** and to overwrite this behavior, `-do` option can be used to display all the external URLs that exist in targets scoped URL / Endpoint.

```
katana -u https://tesla.com -do
```

Here is all the CLI options for the scope control -


```console
katana -h scope

Flags:
SCOPE:
   -cs, -crawl-scope string[]       in scope url regex to be followed by crawler
   -cos, -crawl-out-scope string[]  out of scope url regex to be excluded by crawler
   -fs, -field-scope string         pre-defined scope field (dn,rdn,fqdn) (default "rdn")
   -ns, -no-scope                   disables host based default scope
   -do, -display-out-scope          display external endpoint from scoped crawling
```


## Crawler Configuration

Katana comes with multiple options to configure and control the crawl as the way we want.

*depth*
----

Option to define the `depth` to follow the urls for crawling, the more depth the more number of endpoint being crawled + time for crawl.

```
katana -u https://tesla.com -d 5
```

*js-crawl*
----

Option to enable JavaScript file parsing + crawling the endpoints discovered in JavaScript files, disabled as default.

```
katana -u https://tesla.com -jc
```

*crawl-duration*
----

Option to predefined crawl duration.

```
katana -u https://tesla.com -ct 2
```

*known-files*
----
Option to crawl `robots.txt` and `sitemap.xml` file.

```
katana -u https://tesla.com -kf robotstxt,sitemapxml
```

There are more options to configure when needed, here is all the config related CLI options - 

```console
katana -h config

Flags:
CONFIGURATION:
   -d, -depth int                maximum depth to crawl (default 2)
   -jc, -js-crawl                enable endpoint parsing / crawling in javascript file
   -ct, -crawl-duration int      maximum duration to crawl the target for
   -kf, -known-files string      enable crawling of known files (all,robotstxt,sitemapxml)
   -mrs, -max-response-size int  maximum response size to read (default 2097152)
   -timeout int                  time to wait for request in seconds (default 10)
   -retry int                    number of times to retry the request (default 1)
   -proxy string                 http/socks5 proxy to use
   -H, -headers string[]         custom header/cookie to include in request
   -config string                path to the nuclei configuration file
   -fc, -form-config string      path to custom form configuration file
```

## Field

Katana comes with build in fields that can be used to filter the output for the desired information, `-f` option can be used to specify any of the available fields.

```
   -f, -fields string  field to display in output (url,path,fqdn,rdn,rurl,qurl,qpath,file,key,value,kv,dir,udir)
```

Here is a table with examples of each field and expected output when used - 


| FIELD   | DESCRIPTION                 | EXAMPLE                                                      |
| ------- | --------------------------- | ------------------------------------------------------------ |
| `url`   | URL Endpoint                | `https://admin.projectdiscovery.io/admin/login?user=admin&password=admin` |
| `qurl`  | URL including query param   | `https://admin.projectdiscovery.io/admin/login.php?user=admin&password=admin` |
| `qpath` | Path including query param  | `/login?user=admin&password=admin`                           |
| `path`  | URL Path                    | `https://admin.projectdiscovery.io/admin/login`              |
| `fqdn`  | Fully Qualified Domain name | `admin.projectdiscovery.io`                                  |
| `rdn`   | Root Domain name            | `projectdiscovery.io`                                        |
| `rurl`  | Root URL                    | `https://admin.projectdiscovery.io`                          |
| `file`  | Filename in URL             | `login.php`                                                  |
| `key`   | Parameter keys in URL       | `user,password`                                              |
| `value` | Parameter values in URL     | `admin,admin`                                                |
| `kv`    | Keys=Values in URL          | `user=admin&password=admin`                                  |
| `dir`   | URL Directory name          | `/admin/`                                                    |
| `udir`  | URL with Directory          | `https://admin.projectdiscovery.io/admin/`                   |

Here is an example of using field option to only display all the urls with query parameter in it -

```
katana -u https://tesla.com -f qurl -silent

https://shop.tesla.com/en_au?redirect=no
https://shop.tesla.com/en_nz?redirect=no
https://shop.tesla.com/product/men_s-raven-lightweight-zip-up-bomber-jacket?sku=1740250-00-A
https://shop.tesla.com/product/tesla-shop-gift-card?sku=1767247-00-A
https://shop.tesla.com/product/men_s-chill-crew-neck-sweatshirt?sku=1740176-00-A
https://www.tesla.com/about?redirect=no
https://www.tesla.com/about/legal?redirect=no
https://www.tesla.com/findus/list?redirect=no
```

To compliment `field` option which is useful to filter output at run time, there is `-sf, -store-fields` option which works exactly like field option except instead of filtering, it stores all the information on the disk under `katana_output` directory sorted by target url.

```
katana -u https://tesla.com -sf key,fqdn,qurl -silent
```

```bash
ls katana_output/

https_www.tesla.com_fqdn.txt
https_www.tesla.com_key.txt
https_www.tesla.com_qurl.txt
```

> **Note**: 

> `store-field` option can come handy to collect information to build a target aware wordlist for followings but not limited to - 

- Most / commonly used **parameters**
- Most / commonly used **paths**
- Most / commonly **files**
- Related / unknown **sub(domains)**


## Rate Limit & Delay

It's easy to get blocked / banned while crawling if not following target websites limits, katana comes with multiple option to tune the crawl to go as fast / slow we want.

*delay*
-----

option to introduce a delay in seconds between each new request katana makes while crawling, disabled as default.

*concurrency*
-----
option to control the number of fetchers to use.

*parallelism*
-----
option to define number of target to process at same time from list input.

*rate-limit*
-----
option to use to define max number of request can go out per second.

*rate-limit-minute*
-----
option to use to define max number of request can go out per minute.


Here is all long / short CLI options for rate limit control -

```conosle
katana -h rate-limit

Flags:
RATE-LIMIT:
   -c, -concurrency int          number of concurrent fetchers to use (default 10)
   -p, -parallelism int          number of concurrent inputs to process (default 10)
   -rd, -delay int               request delay between each request in seconds
   -rl, -rate-limit int          maximum requests to send per second (default 150)
   -rlm, -rate-limit-minute int  maximum number of requests to send per minute
```

## Output

*json*
---

Katana support both file output in plain text format as well as JSON which includes additional information like, `source`, `tag`, and `attribute` name to co-related the discovered endpoint.

```console
katana -u https://example.com -json -do | jq .
```

```json
{
  "timestamp": "2022-11-05T22:33:27.745815+05:30",
  "endpoint": "https://www.iana.org/domains/example",
  "source": "https://example.com",
  "tag": "a",
  "attribute": "href"
}
```

Here is additional CLI options related to output -

```conosle
katana -h output

OUTPUT:
   -o, -output string  file to write output to
   -j, -json           write output in JSONL(ines) format
   -nc, -no-color      disable output content coloring (ANSI escape codes)
   -silent             display output only
   -v, -verbose        display verbose output
   -version  
```