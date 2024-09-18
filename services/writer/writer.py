import http.server
import socketserver
import logging
import os
from prometheus_client import start_http_server, Summary

# Prometheus metrics
REQUEST_LATENCY = Summary('request_latency_seconds', 'Latency of HTTP requests in seconds')

LOG_FILE = "/logs/vehicle_summary.log"
PORT = int(os.environ.get("PORT", 8081))

class SimpleHTTPRequestHandler(http.server.BaseHTTPRequestHandler):
    @REQUEST_LATENCY.time()
    def do_POST(self):
        content_length = int(self.headers['Content-Length'])  # Get the size of data
        post_data = self.rfile.read(content_length)  # Read the data
        logging.info("Received POST request: %s", post_data.decode('utf-8'))

        # Log the POST
        with open(LOG_FILE, "a") as log_file:
            log_file.write(post_data.decode('utf-8') + "\n")

        # Send response
        self.send_response(200)
        self.end_headers()

if __name__ == "__main__":
    logging.basicConfig(level=logging.INFO)
    # Prometheus metrics server
    start_http_server(8001)
    with socketserver.TCPServer(("", PORT), SimpleHTTPRequestHandler) as httpd:
        logging.info("Serving on port %d", PORT)
        httpd.serve_forever()