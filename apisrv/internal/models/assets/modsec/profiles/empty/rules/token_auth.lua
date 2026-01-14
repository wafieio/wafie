function main()
    local uri = m.getvar("REQUEST_URI")
    if uri == nil then
        return nil
    end

    -- define the whitelist
    local whitelist = {
       {{{range $index, $path := .Auth.TokenAuth.PathWhitelist}}}
         "{{{ $path }}}",
       {{{end}}}
    }

    -- Skip if URI in whitelist
    for _, pattern in ipairs(whitelist) do
        if string.find(uri, pattern) or string.find(uri, pattern, 1, true) then
            return nil
        end
    end

    -- 2. Define allowed tokens
    local allowedTokens = {
    {{{range $index, $token := .Auth.TokenAuth.Tokens}}}
      "{{{ $token.Token }}}",
    {{{end}}}
    }

    local tokenFromHeader = m.getvar('REQUEST_HEADERS:{{{ default "Authorization" .Auth.TokenAuth.Header  }}}')

    for _, allowedToken in ipairs(allowedTokens) do
        if allowedToken == tokenFromHeader then
            return nil
        end
    end

    -- 3. Block if neither whitelist nor auth matched
    return "Block user"
end