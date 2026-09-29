-- +goose Up
CREATE TABLE permissions (
    id    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    key   TEXT NOT NULL UNIQUE,
    label TEXT NOT NULL
);

-- Fixed, platform-wide catalog — one row per left-nav section (see
-- apps/web/src/components/AppShell.tsx). Studios cannot add their own.
INSERT INTO permissions (key, label) VALUES
    ('dashboard',       'Dashboard'),
    ('inbox',           'Inbox'),
    ('pipeline',        'Pipeline'),
    ('campaigns',       'Campaigns'),
    ('leads',           'Leads'),
    ('social-planner',  'Social Planner'),
    ('payments',        'Payments'),
    ('channels',        'Channels'),
    ('knowledge-base',  'Knowledge Base'),
    ('decision-trees',  'Decision Trees'),
    ('templates',       'Templates'),
    ('settings',        'Settings');

-- +goose Down
DROP TABLE permissions;
