<template>
  <div class="knowledge-edit-page">
    <div class="page-header">
      <button class="btn-back" @click="goBack">← 返回列表</button>
      <h1>{{ isNew ? '新建文章' : '编辑文章' }}</h1>
      <span v-if="article" :class="['status-tag', statusClass(article.status)]">{{ statusText(article.status) }}</span>
    </div>

    <!--  编辑模式下 tab 切换 — 手动编写 / 文档上传 -->
    <div v-if="isNew" class="mode-tabs">
      <button :class="['mode-tab', { active: inputMode === 'manual' }]" @click="inputMode = 'manual'">
        手动编写
      </button>
      <button :class="['mode-tab', { active: inputMode === 'upload' }]" @click="inputMode = 'upload'">
        上传文档
      </button>
    </div>

    <!-- 手动编写模式 -->
    <div v-if="inputMode === 'manual' || !isNew" class="form-card">
      <div class="form-group" v-if="isNew">
        <label>所属知识库 <span class="required">*</span></label>
        <select v-model="form.kb_id" class="form-select">
          <option :value="0" disabled>请选择知识库</option>
          <option v-for="kb in kbList" :key="kb.id" :value="kb.id">{{ kb.name }}</option>
        </select>
      </div>

      <!--  title（替代 question） -->
      <div class="form-group">
        <label>标题 <span class="required">*</span></label>
        <input v-model="form.title" type="text" placeholder="文章标题（如：VPN 配置指南）" :disabled="!canEdit" />
      </div>

      <!--  content（替代 answer）-->
      <div class="form-group">
        <label>内容 <span class="required">*</span></label>
        <textarea v-model="form.content" placeholder="文章正文内容（支持 Markdown）" rows="12" :disabled="!canEdit" />
      </div>

      <div class="form-group">
        <label>分类</label>
        <input v-model="form.category" type="text" placeholder="如：账号管理、网络故障" :disabled="!canEdit" />
      </div>

      <div class="form-group">
        <label>标签</label>
        <div class="tags-input">
          <span v-for="(tag, i) in form.tags" :key="i" class="tag">{{ tag }}<button v-if="canEdit" class="tag-remove" @click="removeTag(i)">×</button></span>
          <input v-if="canEdit" v-model="tagInput" type="text" placeholder="输入标签后按回车" class="tag-input" @keydown.enter.prevent="addTag" />
        </div>
      </div>

      <!-- 编辑操作 -->
      <div class="form-actions" v-if="canEdit && !isReview">
        <button class="btn-save" @click="handleSave" :disabled="saving">{{ isNew ? '创建草稿' : '保存修改' }}</button>
        <button v-if="!isNew" class="btn-submit" @click="handleSubmitReview">提交审核</button>
        <button class="btn-cancel" @click="goBack">取消</button>
      </div>

      <!-- 审核区域 -->
      <div class="review-section" v-if="isReview">
        <h3>审核操作</h3>
        <div class="form-group"><label>审核意见</label><textarea v-model="reviewComment" placeholder="驳回时请填写审核意见" rows="3" /></div>
        <div class="review-actions">
          <button class="btn-approve" @click="handleReview(true)">✓ 通过</button>
          <button class="btn-reject" @click="handleReview(false)">✗ 驳回</button>
        </div>
      </div>

      <!-- 已通过/已发布操作 -->
      <div class="form-actions" v-if="article?.status === 3">
        <button class="btn-publish" @click="handlePublish">发布到 RAG</button>
      </div>
      <div class="form-actions" v-if="article?.status === 4">
        <button class="btn-disable" @click="handleDisable">停用</button>
      </div>
      <div class="form-actions" v-if="article?.status === 5">
        <button class="btn-save" @click="handleEnable">启用</button>
      </div>

      <!-- 处理状态（仅文档上传文章显示） -->
      <div v-if="article?.source_type === 2 && article?.process_status" class="process-info">
        <h4>文档处理状态</h4>
        <span :class="['process-tag', processClass(article.process_status)]">
          {{ processText(article.process_status) }}
        </span>
        <span v-if="article.process_error" class="process-error">{{ article.process_error }}</span>
        <button v-if="article.process_status === 'failed'" class="btn-retry" @click="handleRetryDocument">重试处理</button>
      </div>
    </div>

    <!-- 上传文档模式（仅新建） -->
    <div v-if="isNew && inputMode === 'upload'" class="form-card">
      <div class="form-group">
        <label>所属知识库 <span class="required">*</span></label>
        <select v-model="form.kb_id" class="form-select">
          <option :value="0" disabled>请选择知识库</option>
          <option v-for="kb in kbList" :key="kb.id" :value="kb.id">{{ kb.name }}</option>
        </select>
      </div>

      <div
        class="upload-zone"
        :class="{ 'upload-zone--dragover': isDragover }"
        @dragover.prevent="isDragover = true"
        @dragleave.prevent="isDragover = false"
        @drop.prevent="handleDrop"
        @click="triggerFileInput"
      >
        <input
          ref="fileInput"
          type="file"
          multiple
          accept=".pdf,.docx,.md,.txt"
          class="file-input-hidden"
          @change="handleFileSelect"
        />
        <div class="upload-icon">📄</div>
        <p class="upload-text">拖拽文件到此处或点击选择</p>
        <p class="upload-hint">支持 PDF、DOCX、MD、TXT 格式，单文件 ≤ 50MB</p>
      </div>

      <!-- 待上传文件列表 -->
      <div v-if="uploadFiles.length > 0" class="file-list">
        <div v-for="(f, i) in uploadFiles" :key="i" class="file-item">
          <span class="file-icon">{{ fileIconText(f.name) }}</span>
          <span class="file-name">{{ f.name }}</span>
          <span class="file-size">{{ formatFileSize(f.size) }}</span>
          <button class="file-remove" @click="removeFile(i)">×</button>
        </div>
      </div>

      <div class="form-actions" v-if="uploadFiles.length > 0">
        <button class="btn-save" @click="handleUpload" :disabled="uploading || !form.kb_id">
          {{ uploading ? '上传中...' : `上传 ${uploadFiles.length} 个文件` }}
        </button>
        <button class="btn-cancel" @click="uploadFiles = []; inputMode = 'manual'">取消</button>
      </div>

      <!-- 上传进度提示 -->
      <div v-if="uploadResult" class="upload-result" :class="uploadResult.success ? 'success' : 'error'">
        {{ uploadResult.message }}
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
// TODO(admin/KnowledgeEdit): 组件超过 400 行 — 应拆分为文档上传区域、元信息表单、处理状态展示等子组件。
// TODO(admin/KnowledgeEdit): fetchArticle/fetchKBs 失败时仅 console.error，无用户可见提示，无 loading 状态。
import { ref, computed, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import {
  listKnowledgeBases, getArticleDetail, createArticle, updateArticle,
  submitReview, reviewArticle, publishArticle, disableArticle, enableArticle,
  uploadDocuments, retryDocument,
} from '@/api/knowledge'
import { articleStatusClass as statusClass, articleStatusText as statusText, processClass, processText } from '@/utils/knowledge'

const router = useRouter()
const route = useRoute()
const articleId = route.params.id as string
const kbIdFromQuery = route.query.kb_id as string

const isNew = computed(() => !articleId || articleId === 'new')
const isReview = computed(() => article.value?.status === 2)
const canEdit = computed(() => isNew.value || article.value?.status === 1 || article.value?.status === 6)

// 输入模式（仅新建时可用）
const inputMode = ref<'manual' | 'upload'>('manual')

const kbList = ref<any[]>([])
const article = ref<any>(null)
const saving = ref(false)
const tagInput = ref('')
const reviewComment = ref('')

// title + content 替代 question + answer
const form = ref({
  kb_id: kbIdFromQuery ? Number(kbIdFromQuery) : 0,
  title: '',
  content: '',
  category: '',
  tags: [] as string[],
})

// 文档上传
const fileInput = ref<HTMLInputElement | null>(null)
const uploadFiles = ref<File[]>([])
const isDragover = ref(false)
const uploading = ref(false)
const uploadResult = ref<{ success: boolean; message: string } | null>(null)

onMounted(async () => {
  await fetchKBs()
  if (!isNew.value) await fetchArticle()
})

const fetchKBs = async () => {
  try { const res = await listKnowledgeBases(); kbList.value = Array.isArray(res.data) ? res.data : [] } catch (e) { console.error(e); alert('加载知识库列表失败') }
}
const fetchArticle = async () => {
  try {
    const res = await getArticleDetail(Number(articleId))
    article.value = res.data
    form.value = {
      kb_id: res.data.kb_id,
      title: res.data.title || '',
      content: res.data.content || '',
      category: res.data.category || '',
      tags: res.data.tags || [],
    }
  } catch (e) { console.error(e); alert('加载文章详情失败') }
}

const addTag = () => { const t = tagInput.value.trim(); if (t && !form.value.tags.includes(t)) form.value.tags.push(t); tagInput.value = '' }
const removeTag = (i: number) => { form.value.tags.splice(i, 1) }
const goBack = () => { router.back() }

const handleSave = async () => {
  if (!form.value.title || !form.value.content) { alert('标题和内容不能为空'); return }
  saving.value = true
  try {
    const payload = {
      title: form.value.title,
      content: form.value.content,
      category: form.value.category,
      tags: form.value.tags,
    }
    if (isNew.value) {
      await createArticle(form.value.kb_id, { ...payload, kb_id: form.value.kb_id, source_type: 1 })
    } else {
      await updateArticle(Number(articleId), payload)
    }
    router.back()
  } catch (e: any) { alert(e?.message || '保存失败') } finally { saving.value = false }
}
const handleSubmitReview = async () => { try { await submitReview(Number(articleId)); await fetchArticle() } catch (e: any) { alert(e?.message) } }
const handleReview = async (approved: boolean) => {
  if (!approved && !reviewComment.value.trim()) { alert('驳回时必须填写审核意见'); return }
  try { await reviewArticle(Number(articleId), { approved, review_comment: reviewComment.value }); router.back() } catch (e: any) { alert(e?.message) }
}
const handlePublish = async () => { try { await publishArticle(Number(articleId)); await fetchArticle() } catch (e: any) { alert(e?.message) } }
const handleDisable = async () => { if (!confirm('确定停用？')) return; try { await disableArticle(Number(articleId)); await fetchArticle() } catch (e: any) { alert(e?.message) } }
const handleEnable = async () => { try { await enableArticle(Number(articleId)); await fetchArticle() } catch (e: any) { alert(e?.message) } }
// 文档处理重试
const handleRetryDocument = async () => {
  if (!article.value) return
  try {
    await retryDocument(article.value.kb_id, article.value.id)
    await fetchArticle()
  } catch (e: any) { alert(e?.message || '重试失败') }
}

// 文件选择
function triggerFileInput() { fileInput.value?.click() }
function handleFileSelect(e: Event) {
  const files = (e.target as HTMLInputElement).files
  if (files) addFiles(Array.from(files))
}
function handleDrop(e: DragEvent) {
  isDragover.value = false
  if (e.dataTransfer?.files) addFiles(Array.from(e.dataTransfer.files))
}
function addFiles(newFiles: File[]) {
  const allowed = ['.pdf', '.docx', '.md', '.txt']
  for (const f of newFiles) {
    const ext = '.' + f.name.split('.').pop()?.toLowerCase()
    if (!allowed.includes(ext)) { alert(`不支持的文件类型: ${f.name}`); continue }
    if (f.size > 50 * 1024 * 1024) { alert(`文件过大: ${f.name}（≤ 50MB）`); continue }
    uploadFiles.value.push(f)
  }
}
function removeFile(i: number) { uploadFiles.value.splice(i, 1) }

async function handleUpload() {
  if (!form.value.kb_id || uploadFiles.value.length === 0) return
  // TODO(admin/KnowledgeEdit): 多文件上传时应显示每个文件的独立状态和失败原因。
  // 当前 uploadResult 只有一条汇总消息，部分成功/部分失败时无法表达。
  uploading.value = true
  uploadResult.value = null
  try {
    const fd = new FormData()
    for (const f of uploadFiles.value) fd.append('files', f)
    await uploadDocuments(form.value.kb_id, fd)
    uploadResult.value = { success: true, message: `${uploadFiles.value.length} 个文件上传成功，将在后台异步处理` }
    uploadFiles.value = []
    // 上传成功后切换到手动模式，提示用户可返回列表查看处理状态
    setTimeout(() => { inputMode.value = 'manual'; uploadResult.value = null }, 2500)
  } catch (e: any) {
    uploadResult.value = { success: false, message: e?.response?.data?.message || e?.message || '上传失败' }
  } finally {
    uploading.value = false
  }
}

function fileIconText(name: string) {
  const ext = name.split('.').pop()?.toLowerCase()
  const icons: Record<string, string> = { pdf: '📕', docx: '📘', md: '📝', txt: '📄' }
  return icons[ext || ''] || '📎'
}
function formatFileSize(bytes: number) {
  if (bytes < 1024) return bytes + ' B'
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB'
  return (bytes / (1024 * 1024)).toFixed(1) + ' MB'
}

// statusClass/statusText/processClass/processText → @/utils/knowledge.ts
</script>

<style scoped>
.knowledge-edit-page { max-width: 800px; margin: 0 auto; padding: 20px 24px; }
.page-header { display: flex; align-items: center; gap: 16px; margin-bottom: 20px; }
.page-header h1 { font-size: 20px; font-weight: 600; color: var(--text-primary); }
.btn-back { padding: 6px 12px; background: var(--bg-elevated); border: 1px solid var(--border-default); color: var(--text-secondary); border-radius: 4px; cursor: pointer; font-size: 13px; }

/* 模式切换 */
.mode-tabs { display: flex; gap: 0; margin-bottom: 20px; border-bottom: 2px solid var(--border-default); }
.mode-tab { padding: 10px 24px; background: none; border: none; color: var(--text-secondary); font-size: 14px; cursor: pointer; font-family: inherit; border-bottom: 2px solid transparent; margin-bottom: -2px; }
.mode-tab.active { color: var(--accent); border-bottom-color: var(--accent); }
.mode-tab:hover { color: var(--text-primary); }

/* 表单 */
.form-card { background: var(--bg-elevated); border: 1px solid var(--border-default); border-radius: 8px; padding: 24px; }
.form-group { margin-bottom: 16px; }
.form-group label { display: block; margin-bottom: 4px; font-size: 14px; color: var(--text-secondary); }
.required { color: var(--tag-rejected-text); }
.form-group input, .form-group textarea, .form-select { width: 100%; padding: 8px 10px; background: var(--bg-subtle); border: 1px solid var(--border-default); border-radius: 4px; color: var(--text-primary); font-size: 14px; resize: vertical; font-family: inherit; }
.form-group input:focus, .form-group textarea:focus, .form-select:focus { outline: none; border-color: var(--accent); }
.form-group input:disabled, .form-group textarea:disabled { opacity: 0.6; }
.form-select { cursor: pointer; }

/* 上传区域 */
.upload-zone { border: 2px dashed var(--border-default); border-radius: 8px; padding: 40px; text-align: center; cursor: pointer; transition: border-color 0.2s, background 0.2s; }
.upload-zone:hover, .upload-zone--dragover { border-color: var(--accent); background: rgba(94, 106, 210, 0.06); }
.upload-icon { font-size: 40px; margin-bottom: 12px; }
.upload-text { font-size: 15px; color: var(--text-primary); margin-bottom: 6px; }
.upload-hint { font-size: 12px; color: var(--text-secondary); }
.file-input-hidden { display: none; }

.file-list { margin-top: 16px; display: flex; flex-direction: column; gap: 8px; }
.file-item { display: flex; align-items: center; gap: 10px; padding: 10px 14px; background: var(--bg-subtle); border-radius: 6px; font-size: 13px; }
.file-icon { font-size: 18px; }
.file-name { flex: 1; color: var(--text-primary); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.file-size { color: var(--text-secondary); font-size: 12px; }
.file-remove { background: none; border: none; color: var(--text-secondary); cursor: pointer; font-size: 16px; padding: 0 4px; }
.file-remove:hover { color: var(--tag-rejected-text); }

.upload-result { margin-top: 16px; padding: 12px 16px; border-radius: 6px; font-size: 14px; }
.upload-result.success { background: rgba(46, 160, 67, 0.1); color: var(--color-success); border: 1px solid rgba(46, 160, 67, 0.2); }
.upload-result.error { background: rgba(248, 81, 73, 0.1); color: var(--tag-rejected-text); border: 1px solid rgba(248, 81, 73, 0.2); }

/* 处理状态 */
.process-info { margin-top: 20px; padding-top: 16px; border-top: 1px solid var(--border-default); display: flex; align-items: center; gap: 12px; flex-wrap: wrap; }
.process-info h4 { font-size: 14px; color: var(--text-primary); }
.process-tag { font-size: 12px; padding: 2px 8px; border-radius: 3px; }
.process-tag.pending { background: var(--tag-pending-bg); color: var(--tag-pending-text); }
.process-tag.parsing, .process-tag.embedding { background: var(--tag-pending-bg); color: var(--tag-pending-text); }
.process-tag.completed { background: var(--tag-published-bg); color: var(--tag-published-text); }
.process-tag.failed { background: var(--tag-rejected-bg); color: var(--tag-rejected-text); }
.process-error { font-size: 12px; color: var(--tag-rejected-text); }

.tags-input { display: flex; flex-wrap: wrap; gap: 6px; padding: 6px; background: var(--bg-subtle); border: 1px solid var(--border-default); border-radius: 4px; min-height: 36px; align-items: center; }
.tag { display: inline-flex; align-items: center; gap: 4px; padding: 2px 8px; background: var(--accent); color: #fff; border-radius: 3px; font-size: 12px; }
.tag-remove { background: none; border: none; color: #fff; cursor: pointer; font-size: 14px; padding: 0 2px; }
.tag-input { border: none !important; background: none !important; flex: 1; min-width: 120px; padding: 4px !important; font-size: 13px !important; }

.form-actions { display: flex; gap: 10px; margin-top: 20px; }
.btn-save { padding: 10px 20px; background: var(--accent); color: #fff; border: none; border-radius: 4px; cursor: pointer; font-size: 14px; font-family: inherit; }
.btn-submit { padding: 10px 20px; background: var(--btn-warning-bg); color: var(--btn-warning-text); border: none; border-radius: 4px; cursor: pointer; font-size: 14px; font-family: inherit; }
.btn-cancel { padding: 10px 20px; background: var(--bg-elevated); border: 1px solid var(--border-default); color: var(--text-secondary); border-radius: 4px; cursor: pointer; font-size: 14px; font-family: inherit; }
.btn-approve { padding: 10px 20px; background: var(--btn-success-bg); color: var(--btn-success-text); border: none; border-radius: 4px; cursor: pointer; font-size: 14px; font-family: inherit; }
.btn-reject { padding: 10px 20px; background: var(--btn-danger-bg); color: var(--btn-danger-text); border: none; border-radius: 4px; cursor: pointer; font-size: 14px; font-family: inherit; }
.btn-publish { padding: 10px 20px; background: var(--accent); color: #fff; border: none; border-radius: 4px; cursor: pointer; font-size: 14px; font-family: inherit; }
.btn-disable { padding: 10px 20px; background: var(--btn-danger-bg); color: var(--btn-danger-text); border: none; border-radius: 4px; cursor: pointer; font-size: 14px; font-family: inherit; }
.btn-retry { padding: 8px 16px; background: var(--btn-warning-bg); color: var(--btn-warning-text); border: none; border-radius: 4px; cursor: pointer; font-size: 13px; font-family: inherit; }

.review-section { margin-top: 20px; padding-top: 20px; border-top: 1px solid var(--border-default); }
.review-section h3 { font-size: 16px; color: var(--text-primary); margin-bottom: 12px; }
.review-actions { display: flex; gap: 10px; margin-top: 12px; }

.status-tag { font-size: 12px; padding: 2px 8px; border-radius: 3px; }
.status-tag.draft { background: var(--tag-draft-bg); color: var(--tag-draft-text); }
.status-tag.pending { background: var(--tag-pending-bg); color: var(--tag-pending-text); }
.status-tag.approved { background: var(--tag-approved-bg); color: var(--tag-approved-text); }
.status-tag.published { background: var(--tag-published-bg); color: var(--tag-published-text); }
.status-tag.rejected { background: var(--tag-rejected-bg); color: var(--tag-rejected-text); }
.status-tag.disabled { background: var(--tag-disabled-bg); color: var(--tag-disabled-text); }
</style>
