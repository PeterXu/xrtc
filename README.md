# xRTC

***xRTC*** is an eXtendable WebRTC proxy.

- [x] Serve many WebRTC clients on one ICE port at the same time.
- [x] Serve as an extendable node of WebRTC server (e.g. [Janus](https://github.com/meetecho/janus-gateway)).
- [x] Support `icedirect|icetcp`(transparent) between WebRTC client and server.(partial)

<br>


## 0. How To Use

```
1. Create your local sdp offer,
2. Prepare your sdp answer,
3. Collect your server candidates,

4. Send your ICE to proxy by HTTP-POST '/webrtc/route',
{
	'session_key': 'xxx', 	    // optional
	'offer_ice': {
		'ufrag': 'xxx',			// required
		'pwd': 'xxx',			// required
		'options': 'xxx'		// optional
	},
	'answer_ice': {
		'ufrag': 'xxx',			// required
		'pwd': 'xxx',			// required
		'options': 'xxx'		// optional
	},
	'candidates' : [			// required(1 or 1+)
		'xxx1',
		'xxx2'
	]
}

5. Receive HTTP response from proxy.
{
	'session_key': 'xxx', 	// optional, the same as above
	'candidates' : [		// required
		'xxx1',
		'xxx2'
	]
}

6. Set your offer and anwer to PeerConnection.
7. Set received candidates(from proxy) to your PeerConnection.
8. Then your client may connect to server or proxy (decided by received candidates).
```


<br>

## 1. Cases

### 1). Direct cases

```
WebRTC client A   <---HTTP/WS--->  WebRTC server(Janus/..)
WebRTC client B   <---HTTP/WS--->  WebRTC server(Janus/..)
WebRTC client C   <---HTTP/WS--->  WebRTC server(Janus/..)

WebRTC client A   <---ICE port0--->  WebRTC server(Janus/..)
WebRTC client B   <---ICE port1--->  WebRTC server(Janus/..)
WebRTC client C   <---ICE port2--->  WebRTC server(Janus/..)
```

Two connections: one HTTP/WS connection and another WebRTC-ICE.

The clients must connect to the same WebRTC server directly for interaction.

And some WebRTC server(Janus) need to use different ports for different clients (WebRTC-ICE).

However, most servers only provide limited ports for security.


### 2). xRTC cases

```
WebRTC client A   <--HTTP-RQUEST-->   xRTC0
WebRTC client B   <--HTTP-RQUEST-->   xRTC0
WebRTC client C   <--HTTP-RQUEST-->   xRTC1

WebRTC client A   <--ICE port0-->   xRTC0  <--ICE port1-->  WebRTC server
WebRTC client B   <--ICE port0-->   xRTC0  <--ICE port2-->  WebRTC server
WebRTC client C   <--ICE port1-->   xRTC1  <--ICE port3-->  WebRTC server
```

Two connections: one HTTP-RQUEST connection and another WebRTC-ICE.

The xRTC can use the same port (ICE-UDP/TCP) for different clients(WebRTC-ICE).


<br>

## 2. Flow


### 1) ICE-Hijacked Flow

xRTC is half-proxy for WebRTC connection.

ICE packets(STUN) will be processed seperatly for WebRTC client/server in xRTC proxy.

Data packets(DTLS/SRTP/SRTCP) will be only forwardded between WebRTC client/server.

```
WebRTC client <---------------------->     xRTC    <--------------------> WebRTC server
                                        (1)
              ---- (get ufrag/pwd/.. of offer & answer)
              
                                        (2)
              <--- (return self candidates)
              
                                        (3)
                                connect with server              ------->
              
                       (4)                                     (5)
              <--- ice data0--->       xRTC             <---ice data1--->
              
                                        (6)
              <------         dtls/sctp/srtp/srtcp forward       ------->
```


> ***Step1***: Parse request from WebRTC client.  
> 		xRTC get ice-ufrag/pwd of (offer & answer).
> 
> ***Step2***: Send response to WebRTC client.  
>		xRTC returns self candidates to webrtc client.   
> 
> ***Step3***: Build ice connection between xRTC and WebRTC server.  
> 		xRTC builds ice conenction(send-ice-ufrag/pwd) by libnice.   
> 		The candidates are passive or active-mode.
> 
> ***Step4***: Maintain ice connection0 between WebRTC client and xRTC.  
> 		xRTC makes use of recv-ice-ufrag/pwd to interact with client. 
> 
> ***Step5***: Maintain ice connection1 between xRTC and WebRTC server.  
> 		xRTC-libnice takes the owner to interact with server. 
> 
> ***Step6***: Forward dtls/sctp/srtp/srtcp data between WebRTC client and WebRTC server.  
> 		xRTC only forwards these packets between client and server.


<br>

### 2) ICE-Transparent Flow

xRTC is full-proxy for WebRTC connection (`icedirect: true`).

ICE/Data packets(STUN/DTLS/SRTP/SRTCP) will be forwardded between WebRTC client/server.

```
WebRTC client <---------------------->     xRTC    <--------------------> WebRTC server
                                        (1)
              ---- (get ufrag/pwd/.. of offer & answer)
              
                                        (2)
              <--- (return self candidates)
              
                                        (3)
                                connect with server              ------->
              
                                        (4)
              <--------------    ice data forward    ------------------->
              
                                        (5)
              <------         dtls/sctp/srtp/srtcp forward       ------->
```

> ***Step1***: Parse request from WebRTC client.  
> 		xRTC get ice-ufrag/pwd of (offer & answer).
> 
> ***Step2***: Send response to WebRTC client.  
>		xRTC returns self candidates to webrtc client.   
> 
> ***Step3***: Build connection between xRTC and WebRTC server.  
> 		xRTC builds general network conenction(udp/tcp) with address from Step2. 
> 
> ***Step4***: Foward ice data between WebRTC client and xRTC.  
> 		xRTC only forwards these packets between client and server. 
> 
> ***Step5***: Forward dtls/sctp/srtp/srtcp data between WebRTC client and WebRTC server.  
> 		xRTC only forwards these packets between client and server.


<br>

## 3. Config

The default routes config is [routes.yml](scripts/routes.yml) (YAML format).

The root node is `services` and its structure:

```yaml
services:
  servicename:
    proto: http/tcp/udp
    net:
      addr: :6443
      tls_crt_file: /tmp/etc/cert.pem
      tls_key_file: /tmp/etc/cert.key
      enable_ice: true
      candidate_hosts:
        - candidate_host_ip
    enable_http: true
    http:
      servername: _
      root: /tmp/html
```

Each service is a function(servicename), e.g. udp ice server, tcp ice server or http server.

The server's fields contains:

1. ***proto***: *http/tcp/udp*  
	*http* is a HTTP-REST server,  
	*tcp* is a WebRTC-ICE-TCP or HTTP-REST server,  
	*udp* is a WebRTC-ICE-UDP server,  

2. ***net***: network config, only valid for `proto: udp/tcp/http`
	* ***addr***: server listen address, format: "*ip:port*"
	* ***tls\_crt\_file***: local crt file(openssl)
	* ***tls\_key\_file***: local key file(openssl)
	* ***enable_ice***: *true/false*, enable ice service, only valid for `proto: udp/tcp`.
	* ***candidate_hosts***: server ICE candidate ip/host address, only valid for `proto: udp/tcp`
	
	The `enable` is only valid for `proto: udp/tcp`, for ICE candidates.  
	The `tls_crt_file/tls_key_file` is only valid for `proto: udp/tcp`.  
	The `ips` is only valid for `proto: udp/tcp`, IP of ICE candidates.  
	if `enable` is true, the candidates are constructed by `ips` and port of `addr`.  
	if no valid tls key/crt, http enabled, otherwise both http(ws) and https(wss) are enabled.  
	
3. ***enable_http***: *true/false*, only valid for `proto: tcp`.  
	when *enable_http* is true and current is a tcp server, then it also act as a HTTP-REST server. 
	
4. ***http***: HTTP server config
	* ***servername***: HTTP server name, default "_" for any.  
		if not "_", only matched request will be processsed, like nginx. 
	* ***root***: HTTP static directory for no-routing http request.


<br>

## 4. Build & Run

Simply building for all platforms which support docker:
    
```
$> make docker-pull
$> make docker-mac
$> make deploy-mac
```

<br>

If you want to build completely, following steps as:


1. Library dependency
	
	libffi, libuuid, glib2, libnice, gnutls, openssl
	
2. Routing config

	```
	$> vim scripts/routes.yml
	```
	
3. Common Build for Linux/Mac

	```
	$> make
	$> cp scripts/routes.yml /tmp/etc/routes.yml
	$> make run
	```
	
	
4. Docker Build for CentOS-7
	
	This only works when Step-3 are successful in CentOS-7.
	
	1). Build and Deploy
	
	``` 
	$> make docker
	$> make deploy
	```
	
	2). Generate docker-build image for CentOS-7
	
	```
	$> make docker-build
	```
	
5. Docker Build for Others(Linux/Mac)
	
	This is a cross-platform building and deploying.

	However, it requires the docker image generated in Step-4-(2).
	
	``` 
	$> make docker-mac
	$> make deploy-mac
	```
