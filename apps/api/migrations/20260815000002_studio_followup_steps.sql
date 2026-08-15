-- +goose Up
CREATE TABLE studio_followup_steps (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    studio_id UUID NOT NULL REFERENCES studios(id) ON DELETE CASCADE,
    step_order INT NOT NULL,
    delay_minutes INT NOT NULL,
    message_template TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (studio_id, step_order)
);

-- Seed every existing studio with today's hardcoded no-reply cascade
-- (previously fixed in autocontact_worker.go) so behavior is unchanged on
-- deploy — studios that want to change it now edit it from the admin UI.
INSERT INTO studio_followup_steps (studio_id, step_order, delay_minutes, message_template)
SELECT s.id, f.step_order, f.delay_minutes, f.message_template
FROM studios s
CROSS JOIN (VALUES
    (1, 120,   'Hi {{lead_first_name}}, still thinking about joining {{studio_name}}? We''d love to have you! 💪 Reply *1* to book a trial or *2* to become a member.'),
    (2, 720,   'Hey {{lead_first_name}}! Don''t miss out — spots are limited at {{studio_name}}. Ready to get started? Reply *1* for a trial or *2* for membership.'),
    (3, 1440,  'Hi {{lead_first_name}}, just checking in one more time. We have a great community at {{studio_name}} and we''d love for you to experience it. Reply *1* to book a trial!'),
    (4, 4320,  '{{lead_first_name}}, your spot is still available at {{studio_name}}! 🏋️ Take the first step — reply *1* to book your trial session.'),
    (5, 10080, 'Last follow-up from us, {{lead_first_name}}! If you ever want to start your fitness journey with {{studio_name}}, we''re here for you. Reply *1* anytime to book a trial. 💪')
) AS f(step_order, delay_minutes, message_template);

-- +goose Down
DROP TABLE studio_followup_steps;
