-- +goose Up
--
-- Seeds Mindbody as a crm_provider + its operation mappings, discovered by
-- live-testing against Mindbody's public v6 sandbox (see internal/studios/
-- service.go's SyncLeadToMindbodyByID doc comment for what's wired and why
-- purchase_membership/CheckoutShoppingCart is intentionally left out — it
-- requires staff-level (login-then-bearer) auth the Executor doesn't support
-- yet, and no working staff credential was available to verify it against).
--
-- auth_type = 'api_key': Mindbody's two required headers (Api-Key, SiteId)
-- map directly onto crm.Executor's existing AuthAPIKey case (each configured
-- field becomes its own header, named by its key) — no new auth code needed.
INSERT INTO crm_providers (name, description, base_url, auth_type, auth_field_defs, status)
VALUES (
    'Mindbody',
    'Mindbody Public API v6 — fitness studio management (clients, classes, sales).',
    'https://api.mindbodyonline.com/public/v6',
    'api_key',
    '[
        {"key":"Api-Key","label":"Api-Key","secret":true},
        {"key":"SiteId","label":"Site ID","secret":false}
    ]'::jsonb,
    'active'
);

-- create_lead -> POST /client/addclient. Mindbody requires a fuller profile
-- than our lead model collects (birth date, full address) — SyncLeadToMindbodyByID
-- supplies neutral placeholders for those via the operation's extra params.
-- Response is nested under "Client" (confirmed live: {"Client":{"Id":...},
-- "Status":"Success"}), hence the dot-path response mapping.
INSERT INTO crm_operations (crm_provider_id, operation_key, http_method, path_template, request_mapping, response_mapping, reviewed)
SELECT id, 'create_lead', 'POST', '/client/addclient',
    '{
        "body": {
            "FirstName": "firstName",
            "LastName": "lastName",
            "Email": "email",
            "MobilePhone": "phone",
            "BirthDate": "birthDate",
            "AddressLine1": "street",
            "City": "city",
            "State": "state",
            "PostalCode": "postalCode",
            "ReferredBy": "referredBy"
        }
    }'::jsonb,
    '{"userId": "Client.Id", "email": "Client.Email"}'::jsonb,
    true
FROM crm_providers WHERE name = 'Mindbody';

-- find_membership_plan -> GET /sale/services. Mindbody has no server-side
-- "find by price" filter (unlike this operation's Glofox equivalent, which
-- does the match server-side) — this just lists all services/prices for
-- the site, and SyncLeadToMindbodyByID does the price match client-side in
-- Go, same as glofox.Client.FindMembershipPlanByPrice does internally.
INSERT INTO crm_operations (crm_provider_id, operation_key, http_method, path_template, request_mapping, response_mapping, reviewed)
SELECT id, 'find_membership_plan', 'GET', '/sale/services', '{}'::jsonb, '{}'::jsonb, true
FROM crm_providers WHERE name = 'Mindbody';

-- get_member -> GET /client/clients?ClientIds={id}. Confirmed live: only
-- works with an explicit ClientIds filter — an unfiltered/SearchText query
-- silently returns zero results on this API, so this operation is only
-- ever useful for a client whose Mindbody id you already have (e.g. the
-- id returned by create_lead above).
INSERT INTO crm_operations (crm_provider_id, operation_key, http_method, path_template, request_mapping, response_mapping, reviewed)
SELECT id, 'get_member', 'GET', '/client/clients',
    '{"query": {"ClientIds": "userId"}}'::jsonb,
    '{"email": "Clients.0.Email", "firstName": "Clients.0.FirstName", "lastName": "Clients.0.LastName", "status": "Clients.0.Status"}'::jsonb,
    true
FROM crm_providers WHERE name = 'Mindbody';

-- +goose Down
DELETE FROM crm_operations WHERE crm_provider_id IN (SELECT id FROM crm_providers WHERE name = 'Mindbody');
DELETE FROM crm_providers WHERE name = 'Mindbody';
