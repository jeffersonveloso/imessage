package api

import (
	"net/http"
)

const openapiSpec = `{
  "openapi": "3.0.3",
  "info": {
    "title": "mautrix-imessage REST API",
    "description": "HTTP API for sending and receiving iMessages directly via rustpush, bypassing Matrix. Includes login flows, messaging, webhooks, and status endpoints.",
    "version": "1.0.0",
    "license": {
      "name": "AGPL-3.0",
      "url": "https://www.gnu.org/licenses/agpl-3.0.html"
    }
  },
  "servers": [
    {
      "url": "http://localhost:8080",
      "description": "Local development"
    }
  ],
  "security": [
    {
      "BearerAuth": []
    }
  ],
  "components": {
    "securitySchemes": {
      "BearerAuth": {
        "type": "http",
        "scheme": "bearer",
        "description": "API key configured in api.api_key"
      }
    },
    "schemas": {
      "ErrorResponse": {
        "type": "object",
        "properties": {
          "error": { "type": "string", "example": "invalid or missing bearer token" },
          "code": { "type": "string", "example": "UNAUTHORIZED" }
        },
        "required": ["error", "code"]
      },
      "StatusResponse": {
        "type": "object",
        "properties": {
          "connected": { "type": "boolean" },
          "handle": { "type": "string", "example": "tel:+15551234567" },
          "all_handles": { "type": "array", "items": { "type": "string" } }
        },
        "required": ["connected"]
      },
      "HandlesResponse": {
        "type": "object",
        "properties": {
          "handles": { "type": "array", "items": { "type": "string" } }
        },
        "required": ["handles"]
      },
      "SendRequest": {
        "type": "object",
        "properties": {
          "to": { "type": "string", "description": "Recipient identifier (tel:+... or mailto:...)", "example": "tel:+15551234567" },
          "text": { "type": "string", "example": "Hello from the API!" },
          "is_sms": { "type": "boolean", "default": false },
          "reply_to": { "type": "string", "description": "UUID of message to reply to" },
          "reply_part": { "type": "string" },
          "effect_id": { "type": "string", "description": "iMessage bubble/screen effect ID (e.g. com.apple.MobileSMS.expressivesend.impact, .gentle, .loud, .invisibleink, com.apple.messages.effect.CKHeartEffect)" },
          "subject": { "type": "string", "description": "Bold subject line displayed above the message body" }
        },
        "required": ["to", "text"]
      },
      "SendMediaRequest": {
        "type": "object",
        "description": "Send media via JSON. Provide either 'data' (base64) or 'url' (server-side fetch), not both. When using 'url', mime_type and filename are auto-detected if omitted.",
        "properties": {
          "to": { "type": "string", "example": "tel:+15551234567" },
          "data": { "type": "string", "description": "Base64-encoded file data (optional if url is set)" },
          "url": { "type": "string", "format": "uri", "description": "URL to fetch the file from server-side (optional if data is set). Max 100 MB." },
          "mime_type": { "type": "string", "example": "image/jpeg", "description": "MIME type. Auto-detected from URL response if omitted." },
          "filename": { "type": "string", "example": "photo.jpg", "description": "Filename. Auto-detected from URL if omitted." },
          "is_sms": { "type": "boolean", "default": false },
          "reply_to": { "type": "string" },
          "reply_part": { "type": "string" },
          "effect_id": { "type": "string", "description": "iMessage bubble/screen effect ID" },
          "subject": { "type": "string", "description": "Bold subject line displayed above the message body" },
          "caption": { "type": "string", "description": "Text caption sent alongside the attachment" }
        },
        "required": ["to"]
      },
      "SendMediaMultipartRequest": {
        "type": "object",
        "description": "Send media via multipart/form-data. The file is uploaded as a binary 'file' field. Max 100 MB.",
        "properties": {
          "file": { "type": "string", "format": "binary", "description": "The media file to send" },
          "to": { "type": "string", "example": "tel:+15551234567" },
          "mime_type": { "type": "string", "description": "MIME type. Auto-detected from upload if omitted." },
          "filename": { "type": "string", "description": "Filename. Auto-detected from upload if omitted." },
          "is_sms": { "type": "string", "enum": ["true", "false"], "default": "false" },
          "reply_to": { "type": "string" },
          "reply_part": { "type": "string" },
          "effect_id": { "type": "string" },
          "subject": { "type": "string" },
          "caption": { "type": "string" }
        },
        "required": ["file", "to"]
      },
      "ReactRequest": {
        "type": "object",
        "properties": {
          "to": { "type": "string", "example": "tel:+15551234567" },
          "target_uuid": { "type": "string", "description": "UUID of the message to react to" },
          "target_part": { "type": "integer", "default": 0 },
          "reaction": { "type": "string", "enum": ["heart", "like", "dislike", "laugh", "emphasize", "question", "emoji"] },
          "emoji": { "type": "string", "description": "Required when reaction is 'emoji'" },
          "remove": { "type": "boolean", "default": false },
          "is_sms": { "type": "boolean", "default": false }
        },
        "required": ["to", "target_uuid", "reaction"]
      },
      "EditRequest": {
        "type": "object",
        "properties": {
          "to": { "type": "string" },
          "target_uuid": { "type": "string", "description": "UUID of the message to edit" },
          "new_text": { "type": "string" },
          "is_sms": { "type": "boolean", "default": false }
        },
        "required": ["to", "target_uuid", "new_text"]
      },
      "UnsendRequest": {
        "type": "object",
        "properties": {
          "to": { "type": "string" },
          "target_uuid": { "type": "string", "description": "UUID of the message to unsend" },
          "is_sms": { "type": "boolean", "default": false }
        },
        "required": ["to", "target_uuid"]
      },
      "TypingRequest": {
        "type": "object",
        "properties": {
          "to": { "type": "string" },
          "typing": { "type": "boolean" },
          "is_sms": { "type": "boolean", "default": false }
        },
        "required": ["to", "typing"]
      },
      "ReadReceiptRequest": {
        "type": "object",
        "properties": {
          "to": { "type": "string" },
          "for_uuid": { "type": "string", "description": "UUID of specific message to mark as read" },
          "is_sms": { "type": "boolean", "default": false }
        },
        "required": ["to"]
      },
      "ValidateRequest": {
        "type": "object",
        "properties": {
          "targets": { "type": "array", "items": { "type": "string" }, "description": "List of identifiers to validate" }
        },
        "required": ["targets"]
      },
      "ValidateResponse": {
        "type": "object",
        "properties": {
          "valid": { "type": "array", "items": { "type": "string" } },
          "invalid": { "type": "array", "items": { "type": "string" } }
        },
        "required": ["valid", "invalid"]
      },
      "SendResponse": {
        "type": "object",
        "properties": {
          "uuid": { "type": "string", "description": "iMessage UUID of the sent message" },
          "status": { "type": "string", "example": "sent" }
        },
        "required": ["uuid", "status"]
      },
      "OkResponse": {
        "type": "object",
        "properties": {
          "status": { "type": "string", "example": "ok" }
        }
      },
      "LoginFlowsResponse": {
        "type": "object",
        "properties": {
          "flows": {
            "type": "array",
            "items": { "$ref": "#/components/schemas/LoginFlowInfo" }
          }
        }
      },
      "LoginFlowInfo": {
        "type": "object",
        "properties": {
          "id": { "type": "string", "example": "external-key" },
          "name": { "type": "string", "example": "Apple ID (External Key)" },
          "description": { "type": "string" }
        }
      },
      "LoginStartRequest": {
        "type": "object",
        "properties": {
          "flow": { "type": "string", "description": "Login flow ID from /login/flows", "example": "external-key" },
          "instance_id": { "type": "string", "description": "Optional caller-provided instance ID echoed in every webhook event" }
        },
        "required": ["flow"]
      },
      "LoginStepRequest": {
        "type": "object",
        "properties": {
          "session_id": { "type": "string" },
          "input": { "type": "object", "additionalProperties": { "type": "string" }, "description": "Key-value pairs matching the field IDs from the current step" }
        },
        "required": ["session_id", "input"]
      },
      "LoginStepResponse": {
        "type": "object",
        "properties": {
          "session_id": { "type": "string" },
          "step_id": { "type": "string", "example": "fi.mau.imessage.login.appleid" },
          "instructions": { "type": "string" },
          "fields": {
            "type": "array",
            "items": { "$ref": "#/components/schemas/LoginField" }
          },
          "complete": { "$ref": "#/components/schemas/LoginComplete" }
        },
        "required": ["session_id", "step_id"]
      },
      "LoginField": {
        "type": "object",
        "properties": {
          "id": { "type": "string", "example": "username" },
          "name": { "type": "string", "example": "Apple ID" },
          "type": { "type": "string", "enum": ["email", "password", "username", "2fa_code", "select", "token", "phone_number"], "example": "email" },
          "options": { "type": "array", "items": { "type": "string" }, "description": "Available choices (for type=select)" }
        },
        "required": ["id", "name"]
      },
      "LoginComplete": {
        "type": "object",
        "properties": {
          "login_id": { "type": "string" },
          "message": { "type": "string", "example": "Successfully logged in to iMessage. Bridge is starting." }
        }
      },
      "WebhookEvent": {
        "type": "object",
        "description": "Event POSTed to the configured webhook_url. The 'category' field groups related event types for easier routing.",
        "properties": {
          "type": { "type": "string", "enum": ["message", "reaction", "typing", "read_receipt", "delivered", "edit", "unsend", "connected", "disconnected"], "description": "Specific event type" },
          "category": { "type": "string", "enum": ["connection", "message", "message_update", "message_receipt"], "description": "Event category: connection (connected/disconnected), message (new incoming messages), message_update (edit/unsend/reaction on existing messages), message_receipt (typing/delivered/read_receipt indicators)" },
          "timestamp": { "type": "integer", "description": "Unix timestamp in milliseconds" },
          "instance_id": { "type": "string", "description": "Caller-provided instance ID echoed in every webhook event" },
          "data": { "type": "object", "description": "Event-specific payload. All message-related events (category != connection) include: participants, group_name, is_group, is_sms. Message events additionally include: uuid, sender, text, subject, reply_to, has_attachment. Update events include: uuid, sender, target_uuid, plus type-specific fields." }
        }
      }
    }
  },
  "paths": {
    "/api/v1/status": {
      "get": {
        "tags": ["Query"],
        "summary": "Connection status",
        "description": "Returns whether an iMessage session is active and the connected handles.",
        "responses": {
          "200": { "description": "Status", "content": { "application/json": { "schema": { "$ref": "#/components/schemas/StatusResponse" } } } },
          "401": { "description": "Unauthorized", "content": { "application/json": { "schema": { "$ref": "#/components/schemas/ErrorResponse" } } } }
        }
      }
    },
    "/api/v1/handles": {
      "get": {
        "tags": ["Query"],
        "summary": "List handles",
        "description": "Returns all registered iMessage handles (phone numbers and emails).",
        "responses": {
          "200": { "description": "Handles list", "content": { "application/json": { "schema": { "$ref": "#/components/schemas/HandlesResponse" } } } },
          "503": { "description": "Not connected", "content": { "application/json": { "schema": { "$ref": "#/components/schemas/ErrorResponse" } } } }
        }
      }
    },
    "/api/v1/validate": {
      "post": {
        "tags": ["Query"],
        "summary": "Validate targets",
        "description": "Checks which identifiers are reachable via iMessage.",
        "requestBody": { "required": true, "content": { "application/json": { "schema": { "$ref": "#/components/schemas/ValidateRequest" } } } },
        "responses": {
          "200": { "description": "Validation result", "content": { "application/json": { "schema": { "$ref": "#/components/schemas/ValidateResponse" } } } }
        }
      }
    },
    "/api/v1/send": {
      "post": {
        "tags": ["Send"],
        "summary": "Send text message",
        "requestBody": { "required": true, "content": { "application/json": { "schema": { "$ref": "#/components/schemas/SendRequest" } } } },
        "responses": {
          "200": { "description": "Message sent", "content": { "application/json": { "schema": { "$ref": "#/components/schemas/SendResponse" } } } },
          "503": { "description": "Not connected" }
        }
      }
    },
    "/api/v1/send-media": {
      "post": {
        "tags": ["Send"],
        "summary": "Send media attachment",
        "description": "Send a media file. Supports three modes: (1) JSON with base64 data field, (2) JSON with url field for server-side fetch, (3) multipart/form-data for direct binary upload. Max file size: 100 MB.",
        "requestBody": { "required": true, "content": { "application/json": { "schema": { "$ref": "#/components/schemas/SendMediaRequest" } }, "multipart/form-data": { "schema": { "$ref": "#/components/schemas/SendMediaMultipartRequest" } } } },
        "responses": {
          "200": { "description": "Attachment sent", "content": { "application/json": { "schema": { "$ref": "#/components/schemas/SendResponse" } } } },
          "415": { "description": "Unsupported content type", "content": { "application/json": { "schema": { "$ref": "#/components/schemas/ErrorResponse" } } } }
        }
      }
    },
    "/api/v1/react": {
      "post": {
        "tags": ["Send"],
        "summary": "Send reaction/tapback",
        "requestBody": { "required": true, "content": { "application/json": { "schema": { "$ref": "#/components/schemas/ReactRequest" } } } },
        "responses": {
          "200": { "description": "Reaction sent", "content": { "application/json": { "schema": { "$ref": "#/components/schemas/SendResponse" } } } }
        }
      }
    },
    "/api/v1/edit": {
      "post": {
        "tags": ["Send"],
        "summary": "Edit a sent message",
        "requestBody": { "required": true, "content": { "application/json": { "schema": { "$ref": "#/components/schemas/EditRequest" } } } },
        "responses": {
          "200": { "description": "Edit sent", "content": { "application/json": { "schema": { "$ref": "#/components/schemas/SendResponse" } } } }
        }
      }
    },
    "/api/v1/unsend": {
      "post": {
        "tags": ["Send"],
        "summary": "Unsend a message",
        "requestBody": { "required": true, "content": { "application/json": { "schema": { "$ref": "#/components/schemas/UnsendRequest" } } } },
        "responses": {
          "200": { "description": "Unsend sent", "content": { "application/json": { "schema": { "$ref": "#/components/schemas/SendResponse" } } } }
        }
      }
    },
    "/api/v1/typing": {
      "post": {
        "tags": ["Send"],
        "summary": "Send typing indicator",
        "requestBody": { "required": true, "content": { "application/json": { "schema": { "$ref": "#/components/schemas/TypingRequest" } } } },
        "responses": {
          "200": { "description": "OK", "content": { "application/json": { "schema": { "$ref": "#/components/schemas/OkResponse" } } } }
        }
      }
    },
    "/api/v1/read-receipt": {
      "post": {
        "tags": ["Send"],
        "summary": "Send read receipt",
        "requestBody": { "required": true, "content": { "application/json": { "schema": { "$ref": "#/components/schemas/ReadReceiptRequest" } } } },
        "responses": {
          "200": { "description": "OK", "content": { "application/json": { "schema": { "$ref": "#/components/schemas/OkResponse" } } } }
        }
      }
    },
    "/api/v1/login/flows": {
      "get": {
        "tags": ["Login"],
        "summary": "List login flows",
        "description": "Returns available login flows. Use the flow ID to start a login session.",
        "responses": {
          "200": { "description": "Available flows", "content": { "application/json": { "schema": { "$ref": "#/components/schemas/LoginFlowsResponse" } } } }
        }
      }
    },
    "/api/v1/login/start": {
      "post": {
        "tags": ["Login"],
        "summary": "Start login",
        "description": "Initiates a multi-step login session. Returns a session_id and the first step's fields. For external-key flow: the first step asks for the hardware key (base64 from Mac-Hardware-Info). For apple-id flow (macOS only): the first step asks for Apple ID credentials.",
        "requestBody": { "required": true, "content": { "application/json": { "schema": { "$ref": "#/components/schemas/LoginStartRequest" } } } },
        "responses": {
          "200": { "description": "Login session started", "content": { "application/json": { "schema": { "$ref": "#/components/schemas/LoginStepResponse" } } } }
        }
      }
    },
    "/api/v1/login/step": {
      "post": {
        "tags": ["Login"],
        "summary": "Submit login step",
        "description": "Submits user input for the current login step. The response contains either the next step (with new fields to fill) or a complete object indicating successful login. Steps vary by flow but typically include: hardware_key, Apple ID credentials, 2FA code, device selection, device passcode, and handle selection.",
        "requestBody": { "required": true, "content": { "application/json": { "schema": { "$ref": "#/components/schemas/LoginStepRequest" } } } },
        "responses": {
          "200": { "description": "Next step or completion", "content": { "application/json": { "schema": { "$ref": "#/components/schemas/LoginStepResponse" } } } },
          "404": { "description": "Session not found or expired" }
        }
      }
    },
    "/api/v1/logout": {
      "post": {
        "tags": ["Session"],
        "summary": "Disconnect",
        "description": "Disconnects the active iMessage session. The bridge stops sending and receiving messages. Use the login flow to reconnect.",
        "responses": {
          "200": { "description": "Disconnected", "content": { "application/json": { "schema": { "$ref": "#/components/schemas/OkResponse" } } } },
          "503": { "description": "No active session", "content": { "application/json": { "schema": { "$ref": "#/components/schemas/ErrorResponse" } } } }
        }
      }
    },
    "/api/v1/openapi.json": {
      "get": {
        "tags": ["Docs"],
        "summary": "OpenAPI specification",
        "description": "Returns this OpenAPI 3.0 specification as JSON.",
        "security": [],
        "responses": {
          "200": { "description": "OpenAPI spec" }
        }
      }
    }
  },
  "tags": [
    { "name": "Login", "description": "Multi-step authentication flow" },
    { "name": "Session", "description": "Connection management" },
    { "name": "Query", "description": "Connection status and handle lookup" },
    { "name": "Send", "description": "Send messages, media, reactions, and indicators" },
    { "name": "Docs", "description": "API documentation" }
  ]
}`

// handleOpenAPISpec serves the raw OpenAPI JSON spec.
func handleOpenAPISpec(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(openapiSpec))
}

// swaggerUIHTML is a minimal Swagger UI page that loads from CDN.
const swaggerUIHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>mautrix-imessage API</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
  <style>body{margin:0;padding:0} .topbar{display:none}</style>
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
    SwaggerUIBundle({
      url: window.location.origin + "/api/v1/openapi.json",
      dom_id: "#swagger-ui",
      deepLinking: true,
      presets: [SwaggerUIBundle.presets.apis, SwaggerUIBundle.SwaggerUIStandalonePreset],
      layout: "BaseLayout"
    });
  </script>
</body>
</html>`

// handleSwaggerUI serves the Swagger UI page.
func handleSwaggerUI(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(swaggerUIHTML))
}
