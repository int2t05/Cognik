<template>
  <div class="knowledge-page">
    <div class="knowledge-layout">
      <!-- 左侧：知识库列表 -->
      <aside class="kb-sidebar">
        <div class="sidebar-header">
          <h2>知识库</h2>
          <button class="btn-add-kb" @click="showKBDialog = true">+ 新建</button>
        </div>
        <ul class="kb-list">
          <li
            v-for="kb in kbList"
            :key="kb.id"
            :class="['kb-item', { active: selectedKB?.id === kb.id }]"
            @click="selectKB(kb)"
          >
            <span class="kb-name">{{ kb.name }}</span>
            <button class="btn-edit-kb" @click.stop="startEditKB(kb)" title="编辑知识库">✎</button>
          </li>
        </ul>
        <div v-if="kbList.length === 0" class="empty-hint">暂无知识库</div>
      </aside>

      <!-- 右侧：文章列表 -->
      <main class="articles-main">
        <div class="articles-header">
          <h2>{{ selectedKB ? selectedKB.name + ' — 文章' : '请选择知识库' }}</h2>
          <div class="header-actions">
            <select v-model="sourceTypeFilter" @change="fetchArticles" class="status-filter">
              <option :value="-1">全部来源</option>
              <option :value="1">手动创建</option>
              <option :value="2">文档上传</option>
            </select>
            <select v-model="statusFilter" @change="fetchArticles" class="status-filter">
              <option :value="-1">全部状态</option>
              <option :value="1">草稿</option>
              <option :value="2">待审核</option>
              <option :value="3">已通过</option>
              <option :value="4">已发布</option>
              <option :value="5">已停用</option>
              <option :value="6">已驳回</option>
            </select>
            <button v-if="selectedKB" class="btn-add-article" @click="goCreate">+ 新建文章</button>
          </div>
        </div>

        <div v-if="!selectedKB" class="empty-hint">请在左侧选择一个知识库</div>

        <table v-else-if="articles.length > 0" class="article-table">
          <thead>
            <tr><th>标题</th><th>来源</th><th>分类</th><th>状态</th><th>字数</th><th>处理</th><th>更新时间</th><th>操作</th></tr>
          </thead>
          <tbody>
            <tr v-for="a in articles" :key="a.id">
              <td class="title-cell" @click="goEdit(a.id)">{{ a.title || '-' }}</td>
              <td><span class="source-icon">{{ sourceIcon(a.source_type) }}</span></td>
              <td>{{ a.category || '-' }}</td>
              <td><span :class="['status-tag', statusClass(a.status)]">{{ statusText(a.status) }}</span></td>
              <td>{{ a.word_count || '-' }}</td>
              <td>
                <span v-if="a.source_type === 2" :class="['process-tag', processClass(a.process_status)]">
                  {{ processText(a.process_status) }}
                </span>
                <span v-else class="process-tag">-</span>
              </td>
              <td>{{ formatTime(a.updated_at) }}</td>
              <td class="action-cell">
                <button v-if="a.status===1" class="btn-sm btn-primary" @click="handleSubmitReview(a.id)">提交审核</button>
                <button v-if="a.status===2" class="btn-sm btn-primary" @click="goEdit(a.id)">审核</button>
                <button v-if="a.status===3" class="btn-sm btn-success" @click="handlePublish(a.id)">发布</button>
                <button v-if="a.status===4" class="btn-sm btn-warning" @click="handleDisable(a.id)">停用</button>
                <button v-if="a.status===5" class="btn-sm btn-success" @click="handleEnable(a.id)">启用</button>
                <button v-if="a.process_status==='failed'" class="btn-sm btn-warning" @click="handleRetryDocument(a.id)">重试</button>
                <button v-if="a.status===1||a.status===6" class="btn-sm btn-default" @click="goEdit(a.id)">编辑</button>
              </td>
            </tr>
          </tbody>
        </table>

        <div v-else-if="selectedKB && articles.length === 0" class="empty-hint">暂无文章</div>

        <Pagination v-if="selectedKB && total>0" :total="total" v-model:current-page="currentPage" v-model:page-size="pageSize" @update:current-page="fetchArticles" @update:page-size="fetchArticles" />
      </main>
    </div>

    <!-- 编辑知识库对话框 -->
    <div v-if="showEditKBDialog" class="dialog-overlay" @click.self="showEditKBDialog=false">
      <div class="dialog">
        <h3>编辑知识库</h3>
        <div class="form-group"><label>名称</label><input v-model="editKBForm.name" type="text" /></div>
        <div class="form-group"><label>描述</label><textarea v-model="editKBForm.description" rows="3" /></div>
        <div class="dialog-actions">
          <button class="btn-cancel" @click="showEditKBDialog=false">取消</button>
          <button class="btn-primary" @click="handleUpdateKB" :disabled="!editKBForm.name">保存</button>
        </div>
      </div>
    </div>

    <!-- 新建知识库对话框 -->
    <div v-if="showKBDialog" class="dialog-overlay" @click.self="showKBDialog=false">
      <div class="dialog">
        <h3>新建知识库</h3>
        <div class="form-group"><label>名称</label><input v-model="newKB.name" type="text" /></div>
        <div class="form-group"><label>描述</label><textarea v-model="newKB.description" rows="3" /></div>
        <div class="dialog-actions">
          <button class="btn-cancel" @click="showKBDialog=false">取消</button>
          <button class="btn-primary" @click="handleCreateKB" :disabled="!newKB.name">创建</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import Pagination from '@/components/common/Pagination.vue'
import { articleStatusClass as statusClass, articleStatusText as statusText, processClass, processText } from '@/utils/knowledge'
import { useToast } from '@/composables/useToast'
import { listKnowledgeBases, createKnowledgeBase, updateKnowledgeBase, listArticles, submitReview, publishArticle, disableArticle, enableArticle, retryDocument } from '@/api/knowledge'

interface KB { id: number; name: string }
// 统一文章模型字段
interface Article { id: number; title: string; content: string; category?: string; status: number; source_type: number; word_count?: number; process_status?: string; updated_at?: string }

const router = useRouter()
const kbList = ref<KB[]>([])
const selectedKB = ref<KB | null>(null)
const articles = ref<Article[]>([])
const total = ref(0)
const currentPage = ref(1)
const pageSize = ref(10)
const statusFilter = ref(-1)
const sourceTypeFilter = ref(-1)  // 来源类型筛选
const showKBDialog = ref(false)
const showEditKBDialog = ref(false)
const editingKB = ref<KB | null>(null)
const newKB = ref({ name: '', description: '' })
const editKBForm = ref({ name: '', description: '' })
const toast = useToast()

const hasProcessingDocs = computed(() => articles.value.some(a => a.process_status && a.process_status !== 'completed' && a.process_status !== 'failed'))
let pollTimer: ReturnType<typeof setInterval> | null = null

onMounted(() => { fetchKBs() })
onUnmounted(() => { if (pollTimer) clearInterval(pollTimer) })

// 文档处理状态轮询（每 5 秒刷新正在处理中的文档）
function startDocPolling() {
  if (pollTimer) return
  pollTimer = setInterval(() => {
    if (hasProcessingDocs.value) fetchArticles()
    else { clearInterval(pollTimer!); pollTimer = null }
  }, 5000)
}

const fetchKBs = async () => {
  try { const res = await listKnowledgeBases(); kbList.value = Array.isArray(res.data) ? res.data : [] } catch (e) { console.error(e); toast.showToast('加载知识库列表失败', 'error') }
}
const selectKB = (kb: KB) => { selectedKB.value = kb; currentPage.value = 1; fetchArticles() }
const fetchArticles = async () => {
  if (!selectedKB.value) return
  try {
    const params: any = { page: currentPage.value, page_size: pageSize.value }
    if (statusFilter.value !== -1) params.status = statusFilter.value
    if (sourceTypeFilter.value !== -1) params.source_type = sourceTypeFilter.value
    const res = await listArticles(selectedKB.value.id, params)
    const list = Array.isArray(res.data) ? res.data : ((res.data as any).articles || (res.data as any).items || [])
    articles.value = list; total.value = (res as any).total || list.length || 0
    startDocPolling()
  } catch (e) { console.error(e); toast.showToast('加载文章列表失败', 'error') }
}
const startEditKB = (kb: KB) => { editingKB.value = kb; editKBForm.value = { name: kb.name, description: '' }; showEditKBDialog.value = true }
const handleCreateKB = async () => {
  try { await createKnowledgeBase(newKB.value); showKBDialog.value = false; newKB.value = { name: '', description: '' }; await fetchKBs() } catch (e: any) { alert(e?.message || '创建失败') }
}
const handleUpdateKB = async () => {
  if (!editingKB.value) return
  try { await updateKnowledgeBase(editingKB.value.id, editKBForm.value); showEditKBDialog.value = false; await fetchKBs() } catch (e: any) { alert(e?.message || '更新失败') }
}
const goCreate = () => { router.push(`/admin/knowledge/new?kb_id=${selectedKB.value!.id}`) }
const goEdit = (id: number) => { router.push(`/admin/knowledge/${id}`) }
const handleSubmitReview = async (id: number) => { try { await submitReview(id); await fetchArticles() } catch (e: any) { alert(e?.message) } }
const handlePublish = async (id: number) => { try { await publishArticle(id); await fetchArticles() } catch (e: any) { alert(e?.message) } }
const handleDisable = async (id: number) => { if (!confirm('确定停用？')) return; try { await disableArticle(id); await fetchArticles() } catch (e: any) { alert(e?.message) } }
const handleEnable = async (id: number) => { try { await enableArticle(id); await fetchArticles() } catch (e: any) { alert(e?.message) } }
const handleRetryDocument = async (id: number) => {
  if (!selectedKB.value) return
  try { await retryDocument(selectedKB.value.id, id); await fetchArticles() } catch (e: any) { alert(e?.message) }
}
// 辅助函数；statusClass/statusText/processClass/processText → @/utils/knowledge.ts
const sourceIcon = (t: number) => { const m: Record<number,string> = { 1:'✍️',2:'📄' }; return m[t]||'❓' }
const formatTime = (t?: string) => t ? new Date(t).toLocaleString('zh-CN') : '-'
</script>

<style scoped>
.knowledge-page { height: 100%; }
.knowledge-layout { display: flex; height: 100%; }
.kb-sidebar { width: 220px; min-width: 220px; border-right: 1px solid var(--border-default); padding: 16px; overflow-y: auto; }
.sidebar-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 12px; }
.sidebar-header h2 { font-size: 15px; font-weight: 600; color: var(--text-primary); }
.btn-add-kb { padding: 4px 10px; font-size: 12px; background: var(--accent); color: #fff; border: none; border-radius: 4px; cursor: pointer; }
.kb-list { list-style: none; padding: 0; margin: 0; }
.kb-item { padding: 10px 12px; border-radius: 6px; cursor: pointer; display: flex; justify-content: space-between; align-items: center; margin-bottom: 2px; color: var(--text-secondary); font-size: 13px; }
.kb-item:hover { background: var(--bg-subtle); }
.kb-item.active { background: var(--accent); color: #fff; }
.articles-main { flex: 1; padding: 20px 24px; overflow-y: auto; }
.articles-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px; }
.articles-header h2 { font-size: 18px; font-weight: 600; color: var(--text-primary); }
.header-actions { display: flex; gap: 10px; align-items: center; }
.status-filter { padding: 6px 10px; background: var(--bg-elevated); border: 1px solid var(--border-default); color: var(--text-primary); border-radius: 4px; font-size: 13px; }
.btn-add-article { padding: 8px 16px; background: var(--accent); color: #fff; border: none; border-radius: 4px; cursor: pointer; font-size: 13px; }
.article-table { width: 100%; border-collapse: collapse; }
.article-table th, .article-table td { padding: 10px 12px; text-align: left; border-bottom: 1px solid var(--border-default); font-size: 13px; color: var(--text-primary); }
.article-table th { font-weight: 600; color: var(--text-secondary); font-size: 12px; }
.title-cell { cursor: pointer; color: var(--accent); max-width: 220px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.title-cell:hover { text-decoration: underline; }
.source-icon { font-size: 14px; }
.action-cell { display: flex; gap: 4px; flex-wrap: wrap; }
.btn-sm { padding: 4px 10px; font-size: 12px; border: none; border-radius: 4px; cursor: pointer; white-space: nowrap; }
.btn-primary { background: var(--accent); color: #fff; }
.btn-success { background: var(--btn-success-bg); color: var(--btn-success-text); }
.btn-warning { background: var(--btn-warning-bg); color: var(--btn-warning-text); }
.btn-default { background: var(--bg-elevated); color: var(--text-secondary); border: 1px solid var(--border-default); }
.btn-cancel { padding: 8px 16px; background: var(--bg-elevated); color: var(--text-secondary); border: 1px solid var(--border-default); border-radius: 4px; cursor: pointer; }
.status-tag { font-size: 12px; padding: 2px 6px; border-radius: 3px; }
.status-tag.draft { background: var(--tag-draft-bg); color: var(--tag-draft-text); }
.status-tag.pending { background: var(--tag-pending-bg); color: var(--tag-pending-text); }
.status-tag.approved { background: var(--tag-approved-bg); color: var(--tag-approved-text); }
.status-tag.published { background: var(--tag-published-bg); color: var(--tag-published-text); }
.status-tag.rejected { background: var(--tag-rejected-bg); color: var(--tag-rejected-text); }
.status-tag.disabled { background: var(--tag-disabled-bg); color: var(--tag-disabled-text); }
.process-tag { font-size: 12px; padding: 2px 6px; border-radius: 3px; }
.process-tag.pending { background: var(--tag-pending-bg); color: var(--tag-pending-text); }
.process-tag.completed { background: var(--tag-published-bg); color: var(--tag-published-text); }
.process-tag.failed { background: var(--tag-rejected-bg); color: var(--tag-rejected-text); }
.dialog-overlay { position: fixed; inset: 0; background: rgba(0,0,0,0.6); display: flex; align-items: center; justify-content: center; z-index: 100; }
/* overlay uses fixed black — readable in both dark/light themes */
.dialog { background: var(--bg-elevated); border: 1px solid var(--border-default); border-radius: 8px; padding: 24px; width: 400px; }
.dialog h3 { margin-bottom: 16px; font-size: 16px; color: var(--text-primary); }
.form-group { margin-bottom: 12px; }
.form-group label { display: block; margin-bottom: 4px; font-size: 13px; color: var(--text-secondary); }
.form-group input, .form-group textarea { width: 100%; padding: 8px 10px; background: var(--bg-subtle); border: 1px solid var(--border-default); border-radius: 4px; color: var(--text-primary); font-size: 13px; resize: vertical; }
.dialog-actions { display: flex; gap: 8px; justify-content: flex-end; margin-top: 16px; }
.empty-hint { padding: 60px; text-align: center; color: var(--text-secondary); font-size: 14px; }
.btn-edit-kb { position: absolute; right: 8px; top: 50%; transform: translateY(-50%); background: none; border: none; color: var(--text-tertiary); cursor: pointer; font-size: 12px; padding: 2px 6px; border-radius: 3px; }
.btn-edit-kb:hover { color: var(--accent); background: var(--bg-subtle); }
.kb-item { position: relative; }
</style>
