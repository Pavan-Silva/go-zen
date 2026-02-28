package zen

import (
    "encoding/json"
    "net/url"
)

// BindForm binds form data to dest, which should be a struct tagged with `form`.
// It uses the standard library; user can extend with a third-party library if needed.
func (c *Context) BindForm(dest interface{}) error {
    if err := c.Request.ParseForm(); err != nil {
        return err
    }
    // simple map to struct using json tags hack
    return mapFormValues(c.Request.Form, dest)
}

// mapFormValues is helper that converts url.Values to a struct via JSON roundtrip.
// It's crude but avoids importing dependencies.
func mapFormValues(values url.Values, dest interface{}) error {
    // convert to map[string]string
    m := make(map[string]string)
    for k, v := range values {
        if len(v) > 0 {
            m[k] = v[0]
        }
    }
    // marshal to JSON then unmarshal into dest
    data, err := json.Marshal(m)
    if err != nil {
        return err
    }
    return json.Unmarshal(data, dest)
}
