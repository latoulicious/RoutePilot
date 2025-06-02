import { db } from '../database'
import { routeConfigs, InsertRouteConfigInput } from '../database/schema'
import { eq } from 'drizzle-orm'

export const getRouteByPath = async (path: string) => {
    return await db.select().from(routeConfigs).where(eq(routeConfigs.path, path)).limit(1)
}

export const createRouteConfig = async (data: InsertRouteConfigInput) => {
    return await db.insert(routeConfigs).values(data)
}

export const updateRouteConfig = async (data: Partial<InsertRouteConfigInput> & { id: number }) => {
    const { id, ...updateData } = data
    return await db.update(routeConfigs).set(updateData).where(eq(routeConfigs.id, id))
}

export const deleteRouteConfig = async (id: number) => {
    return await db.delete(routeConfigs).where(eq(routeConfigs.id, id))
}