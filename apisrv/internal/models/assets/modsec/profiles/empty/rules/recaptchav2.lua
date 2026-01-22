-- Load required modules for HTTP requests and JSON parsing
local http = require("socket.http")
local ltn12 = require("ltn12")
local json = require("cjson")

-- Configuration - store your reCAPTCHA secret key securely
local RECAPTCHA_SECRET_KEY =  "{{{ .AntiBot.CaptchaV2.SecretKey }}}"
local RECAPTCHA_VERIFY_URL = "https://www.google.com/recaptcha/api/siteverify"

function main()
    local post_args = m.getvars("ARGS_POST")
    local recaptcha_token = nil
    for i=1, #post_args do
        local raw_name = post_args[i].name
        local raw_value = post_args[i].value
        if string.find(raw_name, "g%-recaptcha%-response") then
            recaptcha_token = string.match(raw_value, "([^%s%\"]+)")
            break
        end
    end
    if recaptcha_token then
        print("token found: " .. recaptcha_token)
    else
        print("token not found")
    end
    local client_ip = m.getvar("REMOTE_ADDR") or "unknown"
    local verification_result = verify_recaptcha(recaptcha_token, client_ip)
    if verification_result.success then
        print("reCAPTCHA verification successful for IP: " .. client_ip)
        -- inform the engine success in captcha verification
        m.setvar("tx.captcha_verified", "1")
        return nil
    else
        print("reCAPTCHA verification failed for IP: " .. client_ip ..
              " Error: " .. (verification_result.error or "Unknown error"))
        -- inform the engine failure in captcha verification
        m.setvar("tx.captcha_verified", "2")
        return "reCAPTCHA verification failed"
    end
end

function verify_recaptcha(token, client_ip)
    print("Starting reCAPTCHA verification for IP: " .. client_ip)
    local post_data = "secret=" .. url_encode(RECAPTCHA_SECRET_KEY) ..
                     "&response=" .. url_encode(token) ..
                     "&remoteip=" .. url_encode(client_ip)
    local response_body = {}
    local result, status_code = http.request{
        url = RECAPTCHA_VERIFY_URL,
        method = "POST",
        headers = {
            ["Content-Type"] = "application/x-www-form-urlencoded",
            ["Content-Length"] = string.len(post_data),
            ["User-Agent"] = "ModSecurity-reCAPTCHA/1.0"
        },
        source = ltn12.source.string(post_data),
        sink = ltn12.sink.table(response_body)
    }
    if not result then
        print("HTTP request to Google reCAPTCHA API failed: " .. (status_code or "unknown error"))
        return {success = false, error = "Network error communicating with Google"}
    end
    local response_text = table.concat(response_body)
    local success, response_data = pcall(json.decode, response_text)
    if not success then
        print("Failed to parse Google reCAPTCHA response JSON: " .. response_text)
        return {success = false, error = "Invalid response from Google"}
    end
    if response_data.success then
        print("Google reCAPTCHA verification successful")
        return {success = true}
    else
        local error_codes = response_data["error-codes"] or {}
        local error_msg = "Verification failed"
        if #error_codes > 0 then
            error_msg = error_msg .. ": " .. table.concat(error_codes, ", ")
        end
        print("Google reCAPTCHA verification failed: " .. error_msg)
        return {success = false, error = error_msg}
    end
end

-- URL encoding function for POST data
function url_encode(str)
    if str then
        str = string.gsub(str, "\n", "\r\n")
        str = string.gsub(str, "([^%w%-%.%_%~ ])", function(c)
            return string.format("%%%02X", string.byte(c))
        end)
        str = string.gsub(str, " ", "+")
    end
    return str
end