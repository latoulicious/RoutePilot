CREATE TABLE "route_configs" (
	"id" serial PRIMARY KEY NOT NULL,
	"path" varchar(255) NOT NULL,
	"method" varchar(10) NOT NULL,
	"config" jsonb NOT NULL,
	"environment" varchar(50) DEFAULT 'production',
	"created_at" timestamp DEFAULT now(),
	"updated_at" timestamp DEFAULT now()
);
