import { pgTable, varchar, jsonb, timestamp, serial } from 'drizzle-orm/pg-core'
import { eq } from 'drizzle-orm'
import { db } from '.'

export const routeConfigs = pgTable('route_configs', {
  id: serial('id').primaryKey(),
  path: varchar('path', { length: 255 }).notNull(),
  method: varchar('method', { length: 10 }).notNull(),
  config: jsonb('config').notNull(), // stored as JSON
  environment: varchar('environment', { length: 50 }).default('production'),
  createdAt: timestamp('created_at').defaultNow(),
  updatedAt: timestamp('updated_at').defaultNow(),
})

export type RouteConfig = typeof routeConfigs.$inferSelect
export type InsertRouteConfigInput = typeof routeConfigs.$inferInsert

export const updateRouteConfig = async (data: Partial<InsertRouteConfigInput> & { id: number }) => {
    const { id, ...updateData } = data
    return await db.update(routeConfigs).set(updateData).where(eq(routeConfigs.id, id))
}

export const deleteRouteConfig = async (id: number) => {
    return await db.delete(routeConfigs).where(eq(routeConfigs.id, id))
}

