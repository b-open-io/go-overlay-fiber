# Basic Overlay Fiber Example

This is a basic example that demonstrates how to set up a simple overlay server using Go Overlay Fiber. This example
uses SQLite for simplicity and doesn't require complex database configuration.

## Environment Variables

Before running this example, set up the following environment variables:

```bash
# Required: Your server's private key for identity
export SERVER_PRIVATE_KEY="your-private-key-here"

# Required: The HTTPS URL where your server will be accessible
export HOSTING_URL="https://your-domain.com"

# Note: This example uses SQLite with a simple file path (./data.db)
# No database configuration needed - SQLite will create the file automatically

# Optional: MongoDB connection string
export MONGO_URL="mongodb://localhost:27017/overlay"
```

## Running the Example

1. Make sure you're in the project root directory
2. Run the example:

```bash
cd example
go run main.go
```

The server will start on port 8080 by default.

## What This Example Does

1. **Server Setup**: Creates a new overlay server instance with a name, private key, and hosting URL
2. **Port Configuration**: Sets the server to listen on port 8080
3. **Database Connections**:
    - Uses SQLite with a simple file path (./data.db) - no configuration required
    - Connects to MongoDB if MONGO_URL is provided
4. **GASP Sync**: Disables GASP sync for simple local deployments
5. **Engine Configuration**: Sets up the overlay engine with the hosting URL
6. **Server Start**: Launches the HTTP server

## Adding Your Overlay Services

To add your own topic managers and lookup services, add them in the designated section:

```go
// ADD YOUR OVERLAY SERVICES HERE
// Example:
// overlayServer.ConfigureTopicManager("myTopic", myTopicManager)
// overlayServer.ConfigureLookupServiceWithMongo("myService", myServiceFactory)
```

## API Endpoints

Once running, the server provides several endpoints:

- `GET /` - Server status and information
- `GET /health` - Health check endpoint
- `GET /listTopicManagers` - List available topic managers
- `GET /listLookupServiceProviders` - List available lookup services
- `POST /submit` - Submit transactions (placeholder in current phase)
- `POST /lookup` - Lookup queries (placeholder in current phase)

## Next Steps

This basic example gets you started with a minimal overlay server.
To build a production overlay service, you'll want to:

1. Add your custom topic managers
2. Configure lookup services for your specific use case
3. Set up proper database schemas and migrations
4. Configure authentication and security settings
5. Add monitoring and logging