import request from './request'
import type { ReminderChannel } from './reminder'

export interface MedkitOwner {
  id: number
  username: string
  permission: 'owner' | 'view' | 'edit'
  can_edit: boolean
  can_manage_users: boolean
}

export interface MedkitAccess {
  id: number
  owner_id: number
  member_id: number
  permission: 'view' | 'edit'
  member_username: string
}

export interface MedkitContext {
  sharing_enabled: boolean
  current_user: { id: number; username: string }
  owners: MedkitOwner[]
  managed_access: MedkitAccess[]
}

export interface Medicine {
  id: number
  owner_id: number
  name: string
  generic_name: string
  specification: string
  quantity: string
  unit: string
  expiry_date: string
  notes: string
  reminder_id?: number
  reminder_enabled: boolean
  reminder_days: number
  reminder_time: string
  reminder_channels: ReminderChannel[]
  reminder_channel_targets: Record<string, number[]>
  created_at: string
  updated_at: string
}

export interface MedicineInput {
  owner_id: number
  name: string
  generic_name: string
  specification: string
  quantity: string
  unit: string
  expiry_date: string
  notes: string
  reminder_enabled: boolean
  reminder_days: number
  reminder_time: string
  reminder_channels: ReminderChannel[]
  reminder_channel_targets: Record<string, number[]>
}

export function getMedkitContext() {
  return request.get<{ code: number; message?: string; data: MedkitContext }>('/api/medkit/context')
}

export function searchMedkitUsers(q: string) {
  return request.get('/api/medkit/users', { params: { q } })
}

export function getMedkitChannelStatuses(ownerID: number) {
  return request.get('/api/medkit/channels', { params: { owner_id: ownerID } })
}

export function getMedkitAccess() {
  return request.get('/api/medkit/access')
}

export function createMedkitAccess(data: { username: string; permission: 'view' | 'edit' }) {
  return request.post('/api/medkit/access', data)
}

export function deleteMedkitAccess(id: number) {
  return request.delete(`/api/medkit/access/${id}`)
}

export function getMedicines(ownerID: number, search?: string) {
  return request.get('/api/medkit/medicines', { params: { owner_id: ownerID, q: search || undefined } })
}

export function createMedicine(data: MedicineInput) {
  return request.post('/api/medkit/medicines', data)
}

export function updateMedicine(id: number, data: MedicineInput) {
  return request.put(`/api/medkit/medicines/${id}`, data)
}

export function deleteMedicine(id: number) {
  return request.delete(`/api/medkit/medicines/${id}`)
}

export function getMedkitAIConfig() {
  return request.get('/api/medkit/admin/ai')
}

export function saveMedkitAIConfig(data: { enabled: boolean; base_url: string; model: string; api_key?: string; clear_api_key?: boolean }) {
  return request.put('/api/medkit/admin/ai', data)
}

export function parseMedkitText(text: string) {
  return request.post('/api/medkit/ai/parse', { text })
}
