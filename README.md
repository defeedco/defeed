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
  "name": "LLM tools",
  "columns": [
    {
      "size": "full",
      "widgets": [
        {
          "limit": 10,
          "collapse_after": 3,
          "show_thumbnails": true,
          "source_id": "issues/browser-use/browser-use"
        },
        {
          "limit": 10,
          "collapse_after": 3,
          "show_thumbnails": true,
          "source_id": "releases/browser-use/browser-use"
        }
      ]
    }
  ]
}
```

This configuration can be viewed at: [http://localhost:8080/page?config=ewogICJuYW1l...](http://localhost:8080/page?config=ewogICJuYW1lIjogIkxMTSB0b29scyIsCiAgImNvbHVtbnMiOiBbCiAgICB7CiAgICAgICJzaXplIjogImZ1bGwiLAogICAgICAid2lkZ2V0cyI6IFsKICAgICAgICB7CiAgICAgICAgICAibGltaXQiOiAxMCwKICAgICAgICAgICJjb2xsYXBzZV9hZnRlciI6IDMsCiAgICAgICAgICAic2hvd190aHVtYm5haWxzIjogdHJ1ZSwKICAgICAgICAgICJzb3VyY2VfaWQiOiAiaXNzdWVzL2Jyb3dzZXItdXNlL2Jyb3dzZXItdXNlIgogICAgICAgIH0sCiAgICAgICAgewogICAgICAgICAgImxpbWl0IjogMTAsCiAgICAgICAgICAiY29sbGFwc2VfYWZ0ZXIiOiAzLAogICAgICAgICAgInNob3dfdGh1bWJuYWlscyI6IHRydWUsCiAgICAgICAgICAic291cmNlX2lkIjogInJlbGVhc2VzL2Jyb3dzZXItdXNlL2Jyb3dzZXItdXNlIgogICAgICAgIH0KICAgICAgXQogICAgfQogIF0KfQ==)
