# quote
Stock quote service demo

THis was built and tested with Linux Mint 20.10 although it should work with any recent Linux distro and possibly on other OS as well...

## Prerequisite software
The versions listed below are what I used for development and testing - recent earlier versions **should** work since I am not relying on cutting edge features.
- `golang` v1.18.1
- `openssl` v3.0.2
- `minikube` v1.28.0
- `docker` Client: v20.20.22, Server: v20.10.20
- `git` v2.34.1
- `curl` v7.81.0 
- `jq` jq-1.6 (Optional for viewing json in an attractive format)

## Preparation
clone repository
create CA
create certificate
minikube addons enable ingress

### Building docker image
Run `build-docker-image.sh` in the `docker` directory

### Minikube prep
1. Run the following:
`minikube addons list`
2. If the ingress addon is not enabled, run the following:
`minikube addons enable ingress`
3. Create the configmap:
`kubectl create configmap quote-config --from-file=quote_config.txt`


## Running the standalone docker version
After building the docker image, run `run-docker-image.sh` in the `docker` directory.

## Running the minikube version
Navigate to the kubernetes directory
1. Start minikube.  Run the following:
`minikube start`
2. Set the docker environment
`eval $(minikube docker-env)`
3. Determine the minikube IP address:
Run `minikube ip` and add `quote.info` as the minikube IP to the local hosts file.  This is `/etc/hosts` under linux.
4. Load the locally built docker image:
`minikube image load quote:latest`
5. Create the configmap:
`kubectl create -f config.yaml`
6. Create the `API_KEY` secret:
`kubectl apply -f secrets.yaml`
7. Create TLS secrets:
I set things up so both the docker image and the ingress can both work with TLS to test and demo each that way.  It is a more common practice (and simpler) to terminate TLS at the ingress.
`kubectl create secret generic certs --from-file=../certs/server.key --from-file=../certs/certbundle.pem`
8. Create the deployment:
`kubectl apply -f deployment.yaml`
9. Create the service:
`kubectl expose deployment stock-quote --port=8443`
10. Apply the ingress:
`kubectl apply -f quote-ingress.yaml`

# Testing
To obtain the stock quote, run the following:
`curl -k -X GET https://quote.info:8443/quote | jq '.'`
The average closing price is added to the metadata as item 6:
```
 % Total    % Received % Xferd  Average Speed   Time    Time     Time  Current
                                 Dload  Upload   Total   Spent    Left  Speed
100  1255  100  1255    0     0    638      0  0:00:01  0:00:01 --:--:--   638
{
  "Meta Data": {
    "1. Information": "Daily Time Series with Splits and Dividend Events",
    "2. Symbol": "MSFT",
    "3. Last Refreshed": "2022-12-29",
    "4. Output Size": "Compact",
    "5. Time Zone": "US/Eastern",
    "6. Average Close": "237.88"
  },
  "Time Series (Daily)": {
    "2022-12-22": {
      "1. open": "241.255",
      "2. high": "241.99",
      "3. low": "233.87",
      "4. close": "238.19",
      "5. adjusted close": "238.19",
      "6. volume": "28651664",
      "7. dividend amount": "0.0000",
      "8. split coefficient": "1.0"
    },
    ,
    ,
    ,

    "2022-12-29": {
      "1. open": "235.65",
      "2. high": "241.92",
      "3. low": "235.65",
      "4. close": "241.01",
      "5. adjusted close": "241.01",
      "6. volume": "19743126",
      "7. dividend amount": "0.0000",
      "8. split coefficient": "1.0"
    }
  }
}
```
If NDAYS is set larger than 100 it automatically fetches the non-compact data from the server to complete your request.  If you request more data than is availble then an error will be returned.

You may also navigate to `https://quote.info/quote` in a browser window to obtain the data, after accepting the warnings about self-signed certificates.

# Shutdown
To terminate the deployment and remove all components:
1. `kubectl delete service stock-quote`
2. `kubectl delete ingress quote-ingress`
3. `kubectl delete deploy stock-quote`
4. `kubectl delete configmap quote-config`
5. `kubectl delete secret certs`
6. `kubectl delete secret quote-secret`

# Possible future work
- Tweak resource limits (I am being overly genererous with the app)
- Use a real (not self-signed) TLS certificate from a source like Let's encrypt: https://letsencrypt.org/
- Script the launch and teardown
- Create a Helm chart for it
- Live updates to permit changing SYMBOL, NDAYS and TLS certificates without any downtime.  (Use either `kubectl patch secret` and/or `kubectl create secret --dry-run | kubectl apply -f -` BS `kubectl create configmap --dry-run | kubectl apply -f -`)
- I had considered caching requests to go easier on the server, but the returned result may or may not include the current day, depending on whether the exchange is closed and results are avaialble, so even a previous query from an hour ago could be out of date.  It might be possible if you know when today's results will be available relative to GMT and timestamp every query.
- There is a fixed retry if the server does not respond or returns an error, but it would be better to check the return code and use an exponential backoff for temporary issues and and just abort with a fatal log message for unrecoverable errors that a retry won't fix like our credentials getting stale.
- Add options to override NDAYS and SYMBOL from the API
- Move it to the cloud
- Add replicates to the quote service to minimize downtime if this is a critical service that must be up 24/7 if we are deploying in a real cloud instead of minikube on a single server.
