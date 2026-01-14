function main()
    local uri = m.getvar("REQUEST_URI")
    if uri == nil then
        return nil
    end

    -- define the whitelist
    local whitelist = {
       {{{range $index, $path := .Auth.BasicAuth.PathWhitelist}}}
         "{{{ $path }}}",
       {{{end}}}
    }

    -- Skip if URI in whitelist
    for _, pattern in ipairs(whitelist) do
        if string.find(uri, pattern) or string.find(uri, pattern, 1, true) then
            return nil
        end
    end

    -- 2. Define allowed Authorization headers
    local users = {
    {{{range $index, $u := .Auth.BasicAuth.Users}}}
      "Basic {{{ printf "%s:%s" $u.User $u.Pass | b64enc }}}",
    {{{end}}}
    }

    local auth = m.getvar("REQUEST_HEADERS:Authorization")

    for _, user in ipairs(users) do
        if auth == user then
            return nil
        end
    end

    -- 3. Block if neither whitelist nor auth matched
    return "Block user"
end