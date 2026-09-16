-- +goose Up
--
-- Phase 4 of the CRM framework (see 20260912000001_crm_framework.sql):
-- seeds Glofox itself as a crm_provider, hand-defined from
-- internal/integrations/glofox/client.go's existing, already-correct
-- endpoints — not AI-guessed, since we already know Glofox's exact API.
--
-- This migration is intentionally additive only: it does not touch any
-- existing studio's Glofox setup, and no crm_connections row is created
-- here (that requires each studio's real x-api-key/x-glofox-api-token
-- values, which only super-admin can add via the CRM Integrations UI).
-- The application's Glofox call sites (studios/leads/firstsession) are
-- UNCHANGED — they still go through the hardcoded glofox.Client. Seeding
-- the catalog entry here is what lets a super-admin verify the mapping and,
-- once ready, connect a studio and cut that studio over — done deliberately
-- as its own separate, verifiable step rather than a silent flag-flip.
--
-- auth_type = 'api_key': Glofox's three static headers map directly onto
-- crm.Executor's AuthAPIKey case — each field becomes its own header, named
-- by its key, exactly matching Client.addAuth's x-glofox-api-token /
-- x-api-key / x-glofox-branch-id.
INSERT INTO crm_providers (name, description, base_url, auth_type, auth_field_defs, status)
VALUES (
    'Glofox',
    'Glofox REST API — the platform''s original CRM integration, now onboarded through the generic framework.',
    'https://gf-api.aws.glofox.com/prod',
    'api_key',
    '[
        {"key":"x-glofox-api-token","label":"API Token","secret":true},
        {"key":"x-api-key","label":"API Key","secret":true},
        {"key":"x-glofox-branch-id","label":"Branch ID","secret":false}
    ]'::jsonb,
    'active'
);

-- create_lead -> POST /2.1/branches/{branchId}/leads (client.go:130-180).
-- branchId comes from the connection's own x-glofox-branch-id credential,
-- via pathFromCreds, not from a per-call param — see executor.go's
-- buildRequest. Glofox's gender-code translation, birth-date default, and
-- the retry-without-"leads"-object-on-"Contact source not allowed" are
-- caller-side business logic (same as Mindbody's create_lead expects its
-- caller to pre-fill neutral placeholders) — the caller passes already-
-- normalized values in params; this mapping only does field placement.
INSERT INTO crm_operations (crm_provider_id, operation_key, http_method, path_template, request_mapping, response_mapping, reviewed)
SELECT id, 'create_lead', 'POST', '/2.1/branches/{branchId}/leads',
    '{
        "pathFromCreds": {"branchId": "x-glofox-branch-id"},
        "body": {
            "email": "email",
            "first_name": "firstName",
            "last_name": "lastName",
            "phone": "phone",
            "type": "type",
            "lead_status": "leadStatus",
            "birth": "birth",
            "gender": "gender"
        }
    }'::jsonb,
    '{"userId": "entity._id", "email": "entity.email"}'::jsonb,
    true
FROM crm_providers WHERE name = 'Glofox';

-- register_user -> POST /2.0/register (client.go:492-537).
INSERT INTO crm_operations (crm_provider_id, operation_key, http_method, path_template, request_mapping, response_mapping, reviewed)
SELECT id, 'register_user', 'POST', '/2.0/register',
    '{
        "body": {"email": "email", "first_name": "firstName", "last_name": "lastName", "phone": "phone", "password": "password"}
    }'::jsonb,
    '{"userId": "user._id", "email": "user.email"}'::jsonb,
    true
FROM crm_providers WHERE name = 'Glofox';

-- purchase_membership -> POST /2.2/branches/{branchId}/users/{userId}/memberships/{membershipId}/plans/{planCode}/purchase
-- (client.go:251-302). userId/membershipId/planCode are per-call params
-- (from create_lead's result and the studio's membership mapping);
-- branchId is again the connection's own credential.
INSERT INTO crm_operations (crm_provider_id, operation_key, http_method, path_template, request_mapping, response_mapping, reviewed)
SELECT id, 'purchase_membership', 'POST',
    '/2.2/branches/{branchId}/users/{userId}/memberships/{membershipId}/plans/{planCode}/purchase',
    '{
        "pathFromCreds": {"branchId": "x-glofox-branch-id"},
        "path": {"userId": "userId", "membershipId": "membershipId", "planCode": "planCode"},
        "body": {"start_date": "startDate"}
    }'::jsonb,
    '{"status": "status", "invoiceId": "invoice_id"}'::jsonb,
    true
FROM crm_providers WHERE name = 'Glofox';

-- find_membership_plan -> GET /2.0/branches/{branchId}/memberships (client.go:319-381).
-- Like Mindbody's equivalent, this has no server-side "find by price" filter
-- — FindMembershipPlanByPrice does the price/trial-name matching client-side
-- in Go over the full membership list, so the operation itself is just the
-- plain list fetch; response_mapping is intentionally empty (caller reads
-- the raw "data" array itself).
INSERT INTO crm_operations (crm_provider_id, operation_key, http_method, path_template, request_mapping, response_mapping, reviewed)
SELECT id, 'find_membership_plan', 'GET', '/2.0/branches/{branchId}/memberships',
    '{"pathFromCreds": {"branchId": "x-glofox-branch-id"}}'::jsonb,
    '{}'::jsonb,
    true
FROM crm_providers WHERE name = 'Glofox';

-- get_member -> GET /2.0/members/{userId} (client.go:462-489).
INSERT INTO crm_operations (crm_provider_id, operation_key, http_method, path_template, request_mapping, response_mapping, reviewed)
SELECT id, 'get_member', 'GET', '/2.0/members/{userId}',
    '{"path": {"userId": "userId"}}'::jsonb,
    '{"email": "email", "accountEmail": "account_email", "firstName": "first_name", "lastName": "last_name", "phone": "phone", "active": "active"}'::jsonb,
    true
FROM crm_providers WHERE name = 'Glofox';

-- list_bookings -> GET /2.2/branches/{branchId}/bookings (client.go:402-429).
-- Response is {"data": [...]} — response_mapping pulls the whole array out
-- under one key, same trick Mindbody's get_member uses for a nested array.
INSERT INTO crm_operations (crm_provider_id, operation_key, http_method, path_template, request_mapping, response_mapping, reviewed)
SELECT id, 'list_bookings', 'GET', '/2.2/branches/{branchId}/bookings',
    '{"pathFromCreds": {"branchId": "x-glofox-branch-id"}}'::jsonb,
    '{"bookings": "data"}'::jsonb,
    true
FROM crm_providers WHERE name = 'Glofox';

-- +goose Down
DELETE FROM crm_operations WHERE crm_provider_id IN (SELECT id FROM crm_providers WHERE name = 'Glofox');
DELETE FROM crm_providers WHERE name = 'Glofox';
