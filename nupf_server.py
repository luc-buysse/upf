import http.server
import socketserver
import sys
import json
import urllib
import urllib.request as request
import time

from concurrent.futures import ThreadPoolExecutor

def subscribe(content_raw):
    parsed_data = json.dumps(content_raw)

    req = request.Request("http://10.200.0.3:8080/ee-Subscriptions",
        data=str.encode(parsed_data),
        headers={
            "Content-Type": "application/json",
        },
        method='POST')

    # throws urllib.error.HTTPError
    return request.urlopen(req)

def subscribe_period(ip_addr, period, corr):
    content_raw = {
        "subscription":  {
            "eventList": [
                {
                    "type": "USER_DATA_USAGE_MEASURES",
                    "granularityOfMeasurement": "PER_SESSION",
                    "measurementTypes": [
                        "VOLUME_MEASUREMENT",
                        "THROUGHPUT_MEASUREMENT"
                    ]
                },
                {
                    "type": "USER_DATA_USAGE_TRENDS",
                    "granularityOfMeasurement": "PER_SESSION"
                }
            ],
            "eventNotifyUri": "10.200.0.4:9822",
            "notifyCorrelationId": corr,
            "eventReportingMode": {
                "trigger": "PERIODIC",
                "repPeriod": period
            },
            "nfId": "d997d381-a672-47c7-aa02-2de02d6873e0",
            "ueIpAddress": {
                "ipv4Addr": ip_addr
            }
        }
    }

    resp = subscribe(content_raw)
    sub_id = json.loads(resp.read())['subscriptionId']
    resp.close()
    return sub_id

def subscribe_continuous(ip_addr, corr):
    content_raw = {
        "subscription":  {
            "eventList": [
                {
                    "type": "PER_FLOW_USAGE_MEASURES"
                }
            ],
            "eventNotifyUri": "10.200.0.4:9822",
            "notifyCorrelationId": corr,
            "eventReportingMode": {
                "trigger": "THRESHOLD",
                "flowSampling": {
                    "uplinkTimeSampling": "NEVER",
                    "downlinkTimeSampling": "1S",
                    "uplinkCountSampling": "NONE",
                    "downlinkCountSampling": "256"
                }
            },
            "nfId": "d997d381-a672-47c7-aa02-2de02d6873e0",
            "ueIpAddress": {
                "ipv4Addr": ip_addr
            }
        }
    }

    resp = subscribe(content_raw)
    sub_id = json.loads(resp.read())['subscriptionId']
    resp.close()
    return sub_id

def unsubscribe(subscriptionId):
    request.urlopen(request.Request(f"http://10.200.0.3:8080/ee-Subscriptions/{subscriptionId}",
            method='DELETE'))

def volumeToInt(v):
    if isinstance(v, int):
        return v
    elif isinstance(v, str):
        return int(v[:-1])

class RequestHandler(http.server.SimpleHTTPRequestHandler):
    def do_POST(self):
        # Handle POST request
        content_length = int(self.headers['Content-Length'])  # Get the size of data
        post_data = self.rfile.read(content_length)  # Read the data

        data = json.loads(post_data)

        notif = data['notificationItems'][0]
        measurement = notif['userDataUsageMeasurements'][0]
        if notif['eventType'] == "USER_DATA_USAGE_MEASURES":
            ue = ues[corr_to_id[data['correlationId']]]

            if 'volumeMeasurement' in measurement:
                if 'volume' not in ue:
                    for k, v in measurement['volumeMeasurement'].items():
                        ue['volume'][k] = volumeToInt(measurement['volumeMeasurement'][k])
                else:
                    for k, v in measurement['volumeMeasurement'].items():
                        ue['volume'][k] += volumeToInt(measurement['volumeMeasurement'][k])
                print('UE ', ue['ip'], ' : ', measurement['volumeMeasurement'])
            elif 'throughputStatisticsMeasurement' in measurement:
                stats = measurement['throughputStatisticsMeasurement']
                print('Trend report: ', stats)

        elif notif['eventType'] == "PER_FLOW_USAGE_MEASURES":
            ue = ues[corr_to_id[data['correlationId']]]

            flow_direction = measurement['flowInfo']['flowDirection']
            flow_desc = measurement['flowInfo']['flowDescription']
            volume = measurement['volumeMeasurement']

            if flow_desc not in ue['flows']:
                ue['flows'][flow_desc] = {
                    'volume_base': {},
                    'volume': {}
                }

                for k in ['totalVolume', 'totalNbOfPackets']:
                    ue['flows'][flow_desc]['volume_base'][k] = volumeToInt(volume[k])
                    ue['flows'][flow_desc]['volume'][k] = 0
            else:
                for k in ['totalVolume', 'totalNbOfPackets']:
                    ue['flows'][flow_desc]['volume'][k] = volumeToInt(volume[k]) - ue['flows'][flow_desc]['volume_base'][k]

            msg = 'UE ' + ue['ip'] + ' '
            msg += flow_direction + ' ' + flow_desc + ' '
            msg += str(ue['flows'][flow_desc]['volume'])

            print(msg)
        else:
            print('eventType not recognised: ', notif)
        self.send_response(204)
        self.end_headers()
        self.wfile.write(b"")
    
    def log_message(self, format, *args):
        return
        

# Port number to listen on
port = 9822
DEFAULT_NUM_UES = 5

corr_to_id = {}
ues = []

with socketserver.TCPServer(("", port), RequestHandler) as httpd:
    print(f"Serving at port {port}")

    try:
        if len(sys.argv) == 1:
            num_ues = DEFAULT_NUM_UES
        else:
            try:
                num_ues = int(sys.argv[1])
            except ValueError:
                num_ues = DEFAULT_NUM_UES

        ues = [{
                'ip': f'10.45.0.{i}', 
                'flows': {},
                'init': False
            } for i in range(2, 2 + num_ues)]

        with ThreadPoolExecutor(max_workers=8) as executor:
            executor.submit(httpd.serve_forever)

            for i, ue in enumerate(ues):
                ip_addr = ue['ip']

                corr_period = f"ue-period-{ip_addr}"
                corr_cont = f"ue-continuous-{ip_addr}"

                ue['period'] = subscribe_period(ip_addr, 1, corr_period)
                ue['cont'] = subscribe_continuous(ip_addr, corr_cont)

                corr_to_id[corr_period] = i
                corr_to_id[corr_cont] = i

                print("Subscribed with: ", ue['period'])
            
            time.sleep(120)
            
            for ue in ues:
                unsubscribe(ue['period'])
                unsubscribe(ue['cont'])

                print("Unsubscribed to: ", ue['period'])
    except Exception as e:
        print(f"Cannot parse: ", e)



