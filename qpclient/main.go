package main

import (
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"quic-proxy-liu/common"
	"runtime"
	"strings"

	"github.com/elazarl/goproxy"

	log "github.com/liudanking/goutil/logutil"
)

func main() {
	//log.Debug("client")

	var (
		listenAddr     string
		proxyUrl       string
		skipCertVerify bool
		auth           string
		verbose        bool
		printVersion   bool
	)

	flag.StringVar(&listenAddr, "l", ":18080", "listenAddr")
	flag.StringVar(&proxyUrl, "proxy", "", "upstream proxy url")
	flag.BoolVar(&skipCertVerify, "k", false, "skip Cert Verify")
	flag.StringVar(&auth, "auth", "quic-proxy:Go!", "basic auth, format: username:password")
	flag.BoolVar(&verbose, "v", false, "verbose")
	flag.BoolVar(&printVersion, "V", false, "print version")
	flag.Parse()

	if printVersion {
		fmt.Fprintf(os.Stdout, "Quic Client %s (%s %s/%s)\n",
			"1.0", runtime.Version(), runtime.GOOS, runtime.GOARCH)
		os.Exit(0)
	}

	proxy := goproxy.NewProxyHttpServer()
	proxy.Verbose = verbose

	Url, err := url.Parse(proxyUrl)
	if err != nil {
		log.Error("proxyUrl:%s invalid", proxyUrl)
		return
	}
	if Url.Scheme == "https" {
		log.Error("quic-proxy only support http proxy")
		return
	}

	parts := strings.Split(auth, ":")
	if len(parts) != 2 {
		log.Error("auth param invalid")
		return
	}
	username, password := parts[0], parts[1]

	proxy.Tr.Proxy = func(req *http.Request) (*url.URL, error) {
		return url.Parse(proxyUrl)
	}

	dialer := common.NewQuicDialer(skipCertVerify)

	proxy.Tr.Dial = dialer.Dial  //proxy.Tr.Dial 是个在net.Transport结构中定义的一个Func变量，这句话等于用一个Dialer下的Dial方法
								//给其赋值。因为它们两的签名一致。
								//赋值以后就是将tcp拨号换成了quic拨号



	// proxy.ConnectDial = proxy.NewConnectDialToProxy(proxyUrl)
	//这个函数是用来处理第一次建立代理连接时候的CONNECT请求，然后调用上面 proxy.Tr.Dial将TCP改造为quic,然后
	//进行拨号至远端代理服务器。（而远端服务器由proxy.Tr.Proxy赋值）

	//按正常，收到浏览器的的CONNECT请求后，本地proxy应该将目的地址提取出来，然后转换成正常的GET请求。但是此处必须将HTTP包头依然转换成为CONNECT
	//然后利用quic拨号至proxy.Tr.Proxy赋值的远端proxy。然后由它再转换成为正常的GET,拨号至最终的目的地。
	//  firefox-->CONNECT请求--> 本地proxy-----------CONNECT--------->远端proxy-->最终的目的地

	proxy.ConnectDial = proxy.NewConnectDialToProxyWithHandler(proxyUrl,
		SetAuthForBasicConnectRequest(username, password))

	// set basic auth
	proxy.OnRequest().Do(SetAuthForBasicRequest(username, password))

	log.Info("start serving %s", listenAddr)
	log.Error("%v", http.ListenAndServe(listenAddr, proxy)) //proxy为什么可以在这里当做handler？ 答：因为它有ServeHTTP方法。
}
