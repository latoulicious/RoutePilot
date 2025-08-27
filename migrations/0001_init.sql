-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- Tenants table
CREATE TABLE tenants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- API Keys table
CREATE TABLE api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    key_id TEXT NOT NULL UNIQUE,
    secret_hash TEXT NOT NULL,
    secret_enc BYTEA NOT NULL,
    active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    last_used_at TIMESTAMP WITH TIME ZONE
);

-- Flags table
CREATE TYPE flag_type AS ENUM ('boolean', 'json');

CREATE TABLE flags (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    key TEXT NOT NULL,
    description TEXT,
    type flag_type NOT NULL DEFAULT 'boolean',
    enabled BOOLEAN DEFAULT FALSE,
    salt TEXT NOT NULL DEFAULT encode(gen_random_bytes(16), 'hex'),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(tenant_id, key)
);

-- Flag Rules table
CREATE TABLE flag_rules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    flag_id UUID NOT NULL REFERENCES flags(id) ON DELETE CASCADE,
    priority INTEGER NOT NULL,
    rollout INTEGER NOT NULL CHECK (rollout >= 0 AND rollout <= 100),
    variant JSONB,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(flag_id, priority)
);

-- Flag Assignments table
CREATE TABLE flag_assignments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    flag_id UUID NOT NULL REFERENCES flags(id) ON DELETE CASCADE,
    subject_id TEXT NOT NULL,
    bucket INTEGER NOT NULL CHECK (bucket >= 0 AND bucket <= 9999),
    chosen_variant JSONB,
    assigned_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(flag_id, subject_id)
);

-- Experiments table
CREATE TYPE experiment_status AS ENUM ('draft', 'running', 'paused', 'stopped');

CREATE TABLE experiments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    key TEXT NOT NULL,
    flag_id UUID NOT NULL REFERENCES flags(id) ON DELETE CASCADE,
    status experiment_status DEFAULT 'draft',
    traffic INTEGER NOT NULL CHECK (traffic >= 0 AND traffic <= 100),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(tenant_id, key)
);

-- Experiment Variants table
CREATE TABLE experiment_variants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    experiment_id UUID NOT NULL REFERENCES experiments(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    weight INTEGER NOT NULL CHECK (weight >= 0),
    config JSONB,
    UNIQUE(experiment_id, name)
);

-- Outbox table for event publishing
CREATE TABLE outbox (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    topic TEXT NOT NULL,
    key TEXT,
    payload JSONB NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    published_at TIMESTAMP WITH TIME ZONE
);

-- Idempotency Keys table
CREATE TABLE idempotency_keys (
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    key UUID NOT NULL,
    method TEXT NOT NULL,
    path_hash BYTEA NOT NULL,
    status INTEGER NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    PRIMARY KEY(tenant_id, key)
);

-- Indexes for performance
CREATE INDEX idx_api_keys_tenant_id ON api_keys(tenant_id);
CREATE INDEX idx_api_keys_active ON api_keys(active) WHERE active = TRUE;
CREATE INDEX idx_api_keys_key_id ON api_keys(key_id);

CREATE INDEX idx_flags_tenant_id ON flags(tenant_id);
CREATE INDEX idx_flags_tenant_key ON flags(tenant_id, key);
CREATE INDEX idx_flags_enabled ON flags(enabled) WHERE enabled = TRUE;

CREATE INDEX idx_flag_rules_flag_id ON flag_rules(flag_id);
CREATE INDEX idx_flag_rules_priority ON flag_rules(flag_id, priority);

CREATE INDEX idx_flag_assignments_flag_subject ON flag_assignments(flag_id, subject_id);

CREATE INDEX idx_experiments_tenant_id ON experiments(tenant_id);
CREATE INDEX idx_experiments_tenant_key ON experiments(tenant_id, key);
CREATE INDEX idx_experiments_flag_id ON experiments(flag_id);
CREATE INDEX idx_experiments_status ON experiments(status);

CREATE INDEX idx_experiment_variants_experiment_id ON experiment_variants(experiment_id);

CREATE INDEX idx_outbox_unpublished ON outbox(created_at) WHERE published_at IS NULL;
CREATE INDEX idx_outbox_tenant_id ON outbox(tenant_id);

CREATE INDEX idx_idempotency_keys_created_at ON idempotency_keys(created_at);

-- Function to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Triggers for updated_at
CREATE TRIGGER update_flags_updated_at BEFORE UPDATE ON flags
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_flag_rules_updated_at BEFORE UPDATE ON flag_rules
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_experiments_updated_at BEFORE UPDATE ON experiments
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();