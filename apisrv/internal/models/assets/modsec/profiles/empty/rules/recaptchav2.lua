function main()
    print("RUNNING RECAPTCHA v2 script")
    local post_args = m.getvars("ARGS_POST")
    print(post_args)
    for i=1, #post_args do
        local name = post_args[i].name
        local value = post_args[i].value
        -- Example: Check if a specific field exists and log it
        if name == "g-recaptcha-response" then
            m.log(3, "Found reCAPTCHA token in Lua: " .. value)
            -- You can perform logic here, like validating the token length
            if string.len(value) < 10 then
                return "Invalid token length detected by Lua script."
            end
        end
    end
end