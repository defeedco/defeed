# Pulse

Never Miss What Matters.

## Usage

### 1. Setup environment variables

Copy the example environment file and customize the values:
```bash
cp .env.example .env
```

### 2. Run the application
```bash
docker compose up --build
```

### 3. Use the API

Visit http://localhost:8080/docs to interact with the REST API.


### 4. View the UI

The page configuration is specified as a base64 encoded JSON string to the `config` query parameter.

> Sources referenced in `source_id` must be manually created using the REST API.

Here is an example of a page configuration:
```json
{
  "name": "MCP updates in LLM agents",
  "columns": [
    {
      "size": "full",
      "widgets": [
        {
          "limit": 10,
          "collapse_after": 3,
          "show_thumbnails": true,
          "source_id": "issues/browser-use/browser-use",
          "query": "MCP support discussions",
          "min_similarity": 0.2
        }
      ]
    },
    {
      "size": "full",
      "widgets": [
        {
          "limit": 10,
          "collapse_after": 3,
          "show_thumbnails": true,
          "source_id": "releases/browser-use/browser-use",
          "query": "MCP related changes",
          "min_similarity": 0.2
        }
      ]
    }
  ]
}
```

This configuration can be viewed at: [http://localhost:8080/page?config=ewogICJuYW1l...](http://localhost:8080/page?config=ewogICJuYW1lIjogIk1DUCB1cGRhdGVzIGluIExMTSBhZ2VudHMiLAogICJjb2x1bW5zIjogWwogICAgewogICAgICAic2l6ZSI6ICJmdWxsIiwKICAgICAgIndpZGdldHMiOiBbCiAgICAgICAgewogICAgICAgICAgImxpbWl0IjogMTAsCiAgICAgICAgICAiY29sbGFwc2VfYWZ0ZXIiOiAzLAogICAgICAgICAgInNob3dfdGh1bWJuYWlscyI6IHRydWUsCiAgICAgICAgICAic291cmNlX2lkIjogImlzc3Vlcy9icm93c2VyLXVzZS9icm93c2VyLXVzZSIsCiAgICAgICAgICAicXVlcnkiOiAiTUNQIHN1cHBvcnQgZGlzY3Vzc2lvbnMiLAogICAgICAgICAgIm1pbl9zaW1pbGFyaXR5IjogMC4yCiAgICAgICAgfQogICAgICBdCiAgICB9LAogICAgewogICAgICAic2l6ZSI6ICJmdWxsIiwKICAgICAgIndpZGdldHMiOiBbCiAgICAgICAgewogICAgICAgICAgImxpbWl0IjogMTAsCiAgICAgICAgICAiY29sbGFwc2VfYWZ0ZXIiOiAzLAogICAgICAgICAgInNob3dfdGh1bWJuYWlscyI6IHRydWUsCiAgICAgICAgICAic291cmNlX2lkIjogInJlbGVhc2VzL2Jyb3dzZXItdXNlL2Jyb3dzZXItdXNlIiwKICAgICAgICAgICJxdWVyeSI6ICJNQ1AgcmVsYXRlZCBjaGFuZ2VzIiwKICAgICAgICAgICJtaW5fc2ltaWxhcml0eSI6IDAuMgogICAgICAgIH0KICAgICAgXQogICAgfQogIF0KfQ==)
