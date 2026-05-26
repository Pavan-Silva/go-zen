package openapi

import "fmt"

// uiHTML returns the HTML for the documentation UI.
func uiHTML(o *OpenAPI) string {
	specURL := o.cfg.SpecPath
	if o.cfg.RenderUI != nil {
		return o.cfg.RenderUI(specURL)
	}
	return swaggerUIHTML(specURL)
}

func swaggerUIHTML(specURL string) string {
	return fmt.Sprintf(`
		<!DOCTYPE html>
		<html lang="en">
			<head>
				<meta charset="UTF-8">
				<title>API Docs</title>
				<link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5/swagger-ui.css">
				<style>html { box-sizing: border-box; overflow-y: scroll; } *, *::before, *::after { box-sizing: inherit; } body { margin: 0; background: #fafafa; }</style>
			</head>

			<body>
				<div id="swagger-ui"></div>
				<script src="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
				<script>
					SwaggerUIBundle({
						url: %q,
						dom_id: "#swagger-ui",
						presets: [SwaggerUIBundle.presets.apis],
						layout: "BaseLayout"
					});
				</script>
			</body>
		</html>`,
		specURL,
	)
}
