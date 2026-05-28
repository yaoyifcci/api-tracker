// MongoDB initialization script for api-tracker
// Executed by the mongo:7 Docker image on first container creation (empty /data/db).
// The backend also calls ensureIndexes() on every startup, so this script is
// complementary — it pre-creates indexes before the backend first connects.

db = db.getSiblingDB("api-tracker");

db.createCollection("api_requests");

// ESR rule: Equality → Sort → Range
db.api_requests.createIndex({ timestamp: -1 });
db.api_requests.createIndex({ provider: 1, timestamp: -1 });
db.api_requests.createIndex({ status_code: 1, timestamp: -1 });
db.api_requests.createIndex({ "req_headers.X-Claude-Code-Session-Id": 1, timestamp: -1 });

print("api-tracker: collection and indexes created");
