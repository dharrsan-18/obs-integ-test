import json
import os
import queue
import threading
import logging
from mitmproxy import http
from opentelemetry import trace
from opentelemetry.exporter.otlp.proto.grpc.trace_exporter import OTLPSpanExporter
from opentelemetry.sdk.resources import Resource
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import BatchSpanProcessor
from opentelemetry.sdk.trace.sampling import TraceIdRatioBased

# Configure stdout logging
logging.basicConfig(level=logging.INFO, format='%(asctime)s - %(levelname)s - %(message)s')
logger = logging.getLogger(__name__)

# Configuration variables
ALLOWED_HOSTS = os.getenv("ALLOWED_HOSTS", "").split(",")
NUM_WORKERS = int(os.getenv("NUM_WORKERS", 5))
WORK_QUEUE_SIZE = int(os.getenv("WORK_QUEUE_SIZE", 100))
OTEL_EXPORTER_ENDPOINT = os.getenv("OTEL_EXPORTER_ENDPOINT", "localhost:4317")
TRACE_RATIO = float(os.getenv("TRACE_RATIO", 1.0))
MAX_BODY_SIZE_IN_BYTES = 1048576
SENSOR_ID = os.getenv("SENSOR_ID")
SOURCE_NAME = os.getenv("SOURCE_NAME", "proxy")
SENSOR_VERSION = "1.0.0"

#check for mandatory configurations
if not SENSOR_ID:
    raise EnvironmentError("SENSOR_ID environment variable is not set. This UUID is required for trace identification.")
if len(ALLOWED_HOSTS)==0:
    raise EnvironmentError("ALLOWED_HOSTS environment variable is not set. This is mandatory to run the otel_client script.")

#Keep the list of allowed domains handy
domain_filter = [d.strip() for d in ALLOWED_HOSTS if d.strip()]

# Queue for holding gRPC request data. Producer/Consumer pattern
work_queue = queue.Queue(maxsize=WORK_QUEUE_SIZE)

# Set up OpenTelemetry tracing
resource = Resource(attributes={"service.name": "mitmproxy-capture", "sensor.id": SENSOR_ID})
tracer_provider = TracerProvider(resource=resource, sampler=TraceIdRatioBased(TRACE_RATIO))
trace.set_tracer_provider(tracer_provider)
tracer = trace.get_tracer(__name__)

# Set up OTEL Exporter configuration
span_processor = BatchSpanProcessor(OTLPSpanExporter(endpoint=OTEL_EXPORTER_ENDPOINT, insecure=True, timeout=3))
span_processor.max_queue_size = 2048
span_processor.schedule_delay_millis = 5000
span_processor.max_export_batch_size = 512
span_processor.export_timeout_millis = 3000
tracer_provider.add_span_processor(span_processor)

# Worker pool(Consumer) implementation
class Worker(threading.Thread):
    def __init__(self, work_queue):
        super().__init__(daemon=True)
        self.work_queue = work_queue

    #Check for http flow object from Producer
    def run(self):
        while True:
            try:
                flow = self.work_queue.get()
                if flow is None:  # Exit signal
                    break
                self.process_request(flow)
            finally:
                self.work_queue.task_done()

    #Process the http flow object. Send an otel trace
    def process_request(self, flow: http.HTTPFlow):    
        if flow.request.host not in domain_filter:
            logger.info("skipping tracing the domain: %s as the host is not in ALLOWED_HOSTS: %s", flow.request.host, domain_filter)
            return  # Skip processing if domain doesn't match

        req_body_length = len(flow.request.content) > 100
        if req_body_length > MAX_BODY_SIZE_IN_BYTES:
            logger.info("skipping tracing the domain: %s as request size is greater than 1MB: %d bytes", flow.request.host, req_body_length)
            return  # Skip processing if domain doesn't match

        resp_body_length = len(flow.response.content) > 100
        if resp_body_length > MAX_BODY_SIZE_IN_BYTES:
            logger.info("skipping tracing the domain: %s as response size is greater than 1MB: %d bytes", flow.request.host, resp_body_length)
            return  # Skip processing if domain doesn't match

        with tracer.start_as_current_span("request_trace") as span:
            span.set_attribute("http.method", flow.request.method)
            span.set_attribute("http.scheme", flow.request.scheme)
            parts = flow.request.http_version.split("/")
            if len(parts) > 1:
                span.set_attribute("http.flavor", parts[1])
            span.set_attribute("http.host", flow.request.host)
            span.set_attribute("net.host.port", flow.request.port)
            span.set_attribute("http.target", flow.request.path)
            span.set_attribute("net.peer.ip", flow.client_conn.peername[0])
            span.set_attribute("net.peer.port", flow.client_conn.peername[1])
            span.set_attribute("obs_source.name", SOURCE_NAME)
            span.set_attribute("sensor.version", SENSOR_VERSION)
            span.set_attribute("sensor.id", SENSOR_ID)
            span.set_attribute("http.status_code", flow.response.status_code)
            req_headers_dict = dict(flow.request.headers)
            resp_headers_dict = dict(flow.response.headers)
            span.set_attribute("http.request.headers", json.dumps(req_headers_dict))
            span.set_attribute("http.response.headers", json.dumps(resp_headers_dict))
            span.set_attribute("http.request.body", str(flow.request.content))
            span.set_attribute("http.response.body", str(flow.response.content))

            print("\n", "="*50)
            print(f"{flow.request.method} {flow.request.url} {flow.request.http_version}")
            # print(flow.client_conn.peername[0], flow.client_conn.peername[1])

            # print("-"*25 + " request headers " + "-"*25)
            # for k, v in flow.request.headers.items():
            #     print("%-30s: %s" % (k.upper(), v))

            # print("-"*25 + " response headers " + "-"*25)
            # for k, v in flow.response.headers.items():
            #     print("%-30s: %s" % (k.upper(), v))

            # print("-"*25 + " req body (first 100 bytes) " + "-"*25)
            # print(flow.request.content[:100])

            # print("-"*25 + " resp body (first 100 bytes) " + "-"*25)
            # print(flow.response.content[:100])

# Initialize worker threads
workers = [Worker(work_queue) for _ in range(NUM_WORKERS)]
for worker in workers:
    worker.start()        

# Entry point for mitmdump. Mitmproxy addon. This will act as Producer
def response(flow: http.HTTPFlow) -> None:
    try:
        #gracefully wait until queue is free. Wait duration is 10 sec 
        work_queue.put(flow, True, 10.00)
        logger.info("Enqueued request to %s for tracing.", flow.request.host)
    except queue.Full:
        logger.warning("Trace queue is full. Dropping trace data.")

#Graceful shutdown for mitmdump script.
def shutdown():
    logger.info("Shutting down gracefully...")
    work_queue.join()  # Wait for all items in the queue to be processed
    for _ in range(NUM_WORKERS):
        work_queue.put(None)
    for t in workers:
        t.join()
    logger.info("All worker threads have exited.")

# Register the shutdown function to be called when mitmdump exits
import atexit
atexit.register(shutdown)