-- +goose Up
CREATE TABLE studio_roles (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    studio_id   UUID NOT NULL REFERENCES studios(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by  UUID REFERENCES users(id) ON DELETE SET NULL,
    updated_by  UUID REFERENCES users(id) ON DELETE SET NULL,
    UNIQUE (studio_id, name)
);
CREATE INDEX idx_studio_roles_studio ON studio_roles(studio_id);

CREATE TABLE studio_role_permissions (
    role_id       UUID NOT NULL REFERENCES studio_roles(id) ON DELETE CASCADE,
    permission_id UUID NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by    UUID REFERENCES users(id) ON DELETE SET NULL,
    PRIMARY KEY (role_id, permission_id)
);

-- +goose Down
DROP TABLE studio_role_permissions;
DROP TABLE studio_roles;
