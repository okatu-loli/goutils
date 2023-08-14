package request

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/moqsien/goutils/pkgs/gtui"
	utils "github.com/moqsien/goutils/pkgs/gutils"
	nproxy "golang.org/x/net/proxy"
)

type Fetcher struct {
	Url          string
	PostBody     map[string]interface{}
	Timeout      time.Duration
	RetryTimes   int
	Headers      map[string]string
	Proxy        string
	NoRedirect   bool
	client       *resty.Client
	proxyEnvName string
	threadNum    int
	size         int64
	bar          *gtui.ProgressBar
	lock         *sync.Mutex
}

func NewFetcher() *Fetcher {
	return &Fetcher{client: resty.New(), proxyEnvName: "GVC_DEFAULT_PROXY", lock: &sync.Mutex{}}
}

func (that *Fetcher) setHeaders() {
	if that.client != nil || len(that.Headers) > 0 {
		for k, v := range that.Headers {
			that.client = that.client.SetHeader(k, v)
		}
	}
}

func (that *Fetcher) parseProxy() (scheme, host string, port int) {
	if that.Proxy == "" {
		that.Proxy = os.Getenv(that.proxyEnvName)
	}
	if that.Proxy == "" {
		return
	}
	if u, err := url.Parse(that.Proxy); err == nil {
		scheme = u.Scheme
		host = u.Hostname()
		port, _ = strconv.Atoi(u.Port())
		if port == 0 {
			port = 80
		}
	}
	return
}

func (that *Fetcher) SetProxyEnvName(name string) {
	if name != "" {
		that.proxyEnvName = name
	}
}

func (that *Fetcher) SetThreadNum(num int) {
	that.threadNum = num
}

func (that *Fetcher) SetUrl(url string) {
	that.Url = url
}

func (that *Fetcher) setProxy() {
	if that.client != nil && that.Proxy != "" {
		scheme, host, port := that.parseProxy()
		switch scheme {
		case "http", "https":
			that.client = that.client.SetProxy(that.Proxy)
		case "socks5":
			httpClient := that.client.GetClient()
			if dialer, err := nproxy.SOCKS5("tcp", fmt.Sprintf("%s:%d", host, port), nil, nproxy.Direct); err == nil {
				httpClient.Transport = &http.Transport{Dial: dialer.Dial}
			} else {
				gtui.PrintError(err)
			}
		default:
			gtui.PrintError(fmt.Sprintf("Unsupported proxy: %s", that.Proxy))
		}
	}
}

func (that *Fetcher) setMisc() {
	that.setHeaders()
	that.setProxy()
	if that.Timeout > 0 {
		that.client = that.client.SetTimeout(that.Timeout)
	}
	if that.RetryTimes > 0 {
		that.client = that.client.SetRetryCount(that.RetryTimes)
	}
	if that.NoRedirect {
		that.client = that.client.SetRedirectPolicy(resty.NoRedirectPolicy())
	}
}

func (that *Fetcher) RemoveProxy() {
	if that.client != nil {
		that.client.RemoveProxy()
	}
}

func (that *Fetcher) Get() (r *resty.Response) {
	if that.client == nil {
		gtui.PrintError("Client is nil.")
		return
	} else {
		that.setMisc()
	}
	if resp, err := that.client.R().SetDoNotParseResponse(true).Get(that.Url); err != nil {
		fmt.Println(err)
	} else {
		r = resp
	}
	return
}

func (that *Fetcher) parseFilename(fPath string) (fName string) {
	dirPath := filepath.Dir(fPath)
	fName = strings.TrimPrefix(strings.ReplaceAll(fPath, dirPath, ""), string(filepath.Separator))
	return
}

func (that *Fetcher) GetFile(localPath string, force ...bool) (size int64) {
	if that.client == nil {
		gtui.PrintError("Client is nil.")
		return
	} else {
		that.setMisc()
	}
	forceToDownload := false
	if len(force) > 0 && force[0] {
		forceToDownload = true
	}
	if ok, _ := utils.PathIsExist(localPath); ok && !forceToDownload {
		gtui.PrintInfo("File already exists.")
		return 100
	}
	if forceToDownload {
		os.RemoveAll(localPath)
	}
	if res, err := that.client.R().SetDoNotParseResponse(true).Get(that.Url); err == nil {
		outFile, err := os.Create(localPath)
		if err != nil {
			gtui.PrintError(fmt.Sprintf("Cannot open file: %+v", err))
			return
		}
		defer utils.Closeq(outFile)
		defer utils.Closeq(res.RawResponse.Body)
		written, err := io.Copy(outFile, res.RawResponse.Body)
		if err != nil {
			fmt.Println(err)
			return
		}
		size = written
	} else {
		fmt.Println(err)
	}
	return
}

func (that *Fetcher) singleDownload(localPath string) (size int64) {
	if res, err := that.client.R().SetDoNotParseResponse(true).Get(that.Url); err == nil {
		outFile, err := os.Create(localPath)
		if err != nil {
			gtui.PrintError(fmt.Sprintf("Cannot open file: %+v", err))
			return
		}
		defer utils.Closeq(outFile)
		defer utils.Closeq(res.RawResponse.Body)

		// io.Copy reads maximum 32kb size, it is perfect for large file download too
		written, err := io.Copy(io.MultiWriter(outFile, that.bar), res.RawResponse.Body)
		if err != nil {
			gtui.PrintError(err)
			return
		}
		size = written
	} else {
		fmt.Println(err)
	}
	return
}

func (that *Fetcher) getPartFileName(localPath string, id int) string {
	filename := that.parseFilename(localPath)
	filename = fmt.Sprintf("%s.part%v", filename, id)
	return filepath.Join(that.getPartDir(localPath), filename)
}

func (that *Fetcher) getPartDir(localPath string) string {
	return filepath.Join(filepath.Dir(localPath), "temp_part_xxx")
}

func (that *Fetcher) mergeFile(localPath string) error {
	dest_file, err := os.OpenFile(localPath, os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		return err
	}
	defer utils.Closeq(dest_file)

	for i := 0; i < that.threadNum; i++ {
		partfile_name := that.getPartFileName(localPath, i)
		part_file, err := os.Open(partfile_name)
		if err != nil {
			return err
		}
		io.Copy(dest_file, part_file)
		utils.Closeq(part_file)
		os.Remove(partfile_name)
	}
	return nil
}

func (that *Fetcher) partDownload(localPath string, range_begin, range_end, id int) {
	if range_begin >= range_end {
		return
	}

	that.client = resty.New()
	that.setMisc()
	client := that.client
	that.client = nil

	client.SetHeader("Range", fmt.Sprintf("bytes=%d-%d", range_begin, range_end))
	if res, err := client.R().SetDoNotParseResponse(true).Get(that.Url); err == nil {
		outFile, err := os.Create(that.getPartFileName(localPath, id))
		if err != nil {
			gtui.PrintError(fmt.Sprintf("Cannot open file: %+v", err))
			return
		}
		defer utils.Closeq(outFile)
		defer utils.Closeq(res.RawResponse.Body)

		// io.Copy reads maximum 32kb size, it is perfect for large file download too
		written, err := io.Copy(io.MultiWriter(outFile, that.bar), res.RawResponse.Body)
		if err != nil {
			fmt.Println(err)
			return
		}
		that.lock.Lock()
		that.size += written
		that.lock.Unlock()
		if res.RawResponse.StatusCode != 200 && written < int64(range_end-range_begin) {
			gtui.PrintFatal(fmt.Sprintf("Download failed, status code: %d", res.RawResponse.StatusCode))
			gtui.PrintWarning(fmt.Sprintf("Please remove temp files manually: %s.", that.getPartDir(localPath)))
			os.Exit(1)
		}
	} else {
		gtui.PrintError(err)
	}
}

func (that *Fetcher) multiDownload(localPath string, content_size int) error {
	part_size := content_size / that.threadNum

	part_dir := that.getPartDir(localPath)
	os.Mkdir(part_dir, 0777)
	defer os.RemoveAll(part_dir)

	var waitgroup sync.WaitGroup
	waitgroup.Add(that.threadNum)

	range_init := 0

	for i := 0; i < that.threadNum; i++ {
		// concurrency request, i for thread id
		id := i
		go func(i, range_begin int) {
			defer waitgroup.Done()
			range_end := range_begin + part_size
			if i == that.threadNum-1 {
				range_end = content_size
			} // for the last data block
			that.partDownload(localPath, range_begin, range_end, i)
		}(id, range_init)

		range_init += part_size + 1
	}
	waitgroup.Wait()

	// merge
	that.mergeFile(localPath)
	return nil
}

func (that *Fetcher) Download(localPath string, force ...bool) (size int64) {
	if that.client == nil {
		gtui.PrintError("Client is nil.")
		return
	} else {
		that.setMisc()
	}
	forceToDownload := false
	if len(force) > 0 && force[0] {
		forceToDownload = true
	}
	if ok, _ := utils.PathIsExist(localPath); ok && !forceToDownload {
		gtui.PrintInfo("File already exists.")
		return 100
	}
	if forceToDownload {
		os.RemoveAll(localPath)
	}

	var content_length int64
	if res, err := that.client.R().SetDoNotParseResponse(true).Head(that.Url); err == nil {
		content_length = res.RawResponse.ContentLength
		if content_length == 0 {
			gtui.PrintError("Content-Length is zero.")
			return
		}
		that.bar = gtui.NewProgressBar(that.parseFilename(localPath), int(content_length))
		that.bar.Start()
	} else {
		gtui.PrintError(err)
		return
	}

	if that.threadNum <= 1 {
		size = that.singleDownload(localPath)
	} else {
		that.multiDownload(localPath, int(content_length))
		size = that.size
	}
	return
}

func (that *Fetcher) Post() (r *resty.Response) {
	if that.client == nil {
		gtui.PrintError("Client is nil.")
		return
	} else {
		that.setMisc()
	}
	if resp, err := that.client.SetDoNotParseResponse(true).R().SetBody(that.PostBody).Post(that.Url); err != nil {
		fmt.Println(err)
		return
	} else {
		r = resp
	}
	return
}