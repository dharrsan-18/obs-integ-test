function setup()
    print("Setup function called. Performing additional initialization if required.")
end

function init(args)
    local needs = {}
    needs["protocol"] = "http" 
    return needs
end

local json = require "json"

function log(args)
    -- Get all the required data
    local sec, usec = SCPacketTimestamp()
    local ts_string = SCPacketTimeString()
    local ipver, srcip, dstip, proto, sp, dp = SCPacketTuple()
    
    -- Initialize the main structure
    local output = {
        metadata = {
            timestamp = string.format("%d.%06d", sec, usec),
            src_ip = srcip,
            src_port = sp,
            dest_ip = dstip,
            dest_port = dp
        },
        request = {
            header = {},
            body = ""
        },
        response = {
            header = {},
            body = ""
        }
    }

    -- Handle request headers
    output.request.header["request-line"] = HttpGetRequestLine() or ""
    output.request.header["Host"] = HttpGetRequestHost() or ""
    
    -- Get all request headers dynamically
    local request_headers = HttpGetRequestHeaders()
    if request_headers then
        for k, v in pairs(request_headers) do
            output.request.header[k] = v
        end
    end

    -- Handle request body
    local request_body_parts = HttpGetRequestBody()
    if request_body_parts then
        local body = ""
        for _, part in ipairs(request_body_parts) do
            body = body .. part
        end
        output.request.body = body
    else
        output.request.body = ""
    end

    -- Handle response headers
    output.response.header["response-line"] = HttpGetResponseLine() or ""
    
    -- Get all response headers dynamically
    local response_headers = HttpGetResponseHeaders()
    if response_headers then
        for k, v in pairs(response_headers) do
            output.response.header[k] = v
        end
    end

    -- Handle response body
    local response_body_parts = HttpGetResponseBody()
    if response_body_parts then
        local body = ""
        for _, part in ipairs(response_body_parts) do
            body = body .. part
        end
        output.response.body = body
    else
        output.response.body = ""
    end

    -- Print the final JSON output
    print(json.encode(output))
end

-- Function to clean up or log final information when the script is done
function deinit()
    print("Script finished logging packets and HTTP data.")
end
