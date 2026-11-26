<script setup lang="ts">
import { computed, nextTick, ref } from 'vue'
import TreeCanvas, { type TreeNode } from './components/TreeCanvas.vue'
import { BrowseFolder, CopyPath, LoadChildren, OpenInExplorer, OpenPath, Query, RootNode, Scan } from '../wailsjs/go/main/App'

interface ExtensionCount { extension: string; count: number }
interface ScanSummary {
  cancelled: boolean; root: string; rootName: string; files: number
  directories: number; skipped: number; durationMs: number; extensions: ExtensionCount[]
}
interface QueryResult { nodes: TreeNode[]; matches: number; truncated: boolean; queryMs: number }
interface FolderEntry { name: string; path: string }
interface FolderListing { path: string; parent: string; folders: FolderEntry[]; truncated: boolean }

const summary = ref<ScanSummary | null>(null)
const result = ref<QueryResult>({ nodes: [], matches: 0, truncated: false, queryMs: 0 })
const search = ref('')
const selectedExtensions = ref<string[]>([])
const scanning = ref(false)
const querying = ref(false)
const error = ref('')
const notice = ref('')
const showTypes = ref(false)
const pickerOpen = ref(false)
const pickerLoading = ref(false)
const pickerPath = ref('')
const pickerParent = ref('')
const pickerFolders = ref<FolderEntry[]>([])
const pickerTruncated = ref(false)
const viewRoot = ref<TreeNode | null>(null)
const loadedDirectories = new Set<number>()
const loadingDirectories = new Set<number>()
const directoryOffsets = new Map<number, number>()
const partialDirectories = ref(new Set<number>())
const contextMenu = ref<{ node: TreeNode; x: number; y: number } | null>(null)
const maxDepth = ref(0)
const branchFocusID = ref<number | null>(null)
const treeRevision = ref(0)
const depthRevision = ref(0)
let searchTimer = 0
let requestVersion = 0
let noticeTimer = 0

const selectedLabel = computed(() => {
  const count = selectedExtensions.value.length
  return count === 0 ? '全部类型' : count === 1 ? selectedExtensions.value[0] : `${count} 种类型`
})

async function openFolderPicker() {
  pickerOpen.value = true
  await browseFolder(summary.value?.root || '')
}

async function browseFolder(path: string) {
  pickerLoading.value = true
  error.value = ''
  try {
    const value = await BrowseFolder(path) as FolderListing
    pickerPath.value = value.path
    pickerParent.value = value.parent
    pickerFolders.value = value.folders || []
    pickerTruncated.value = value.truncated
  } catch (e) {
    error.value = String(e)
  } finally {
    pickerLoading.value = false
  }
}

async function chooseFolder(path = pickerPath.value) {
  if (!path) return
  pickerOpen.value = false
  scanning.value = true
  error.value = ''
  try {
    // Let Vue paint the loading layer before the native scan call begins.
    await nextTick()
    await new Promise<void>(resolve => requestAnimationFrame(() => resolve()))
    const value = await Scan(path) as ScanSummary
    if (!value.cancelled) {
      summary.value = value
      selectedExtensions.value = []
      search.value = ''
      const root = await RootNode() as TreeNode
      viewRoot.value = root
      branchFocusID.value = null
      loadedDirectories.clear()
      directoryOffsets.clear()
      await runQuery()
    }
  } catch (e) {
    error.value = String(e)
  } finally {
    scanning.value = false
  }
}

async function runQuery() {
  if (!summary.value) return
  const version = ++requestVersion
  querying.value = true
  try {
    const value = await Query({
      text: search.value,
      extensions: selectedExtensions.value,
      limit: 1600,
      rootId: viewRoot.value?.id ?? 0,
      focusId: branchFocusID.value ?? -1,
    }) as QueryResult
    if (version === requestVersion) {
      result.value = value
      treeRevision.value++
    }
  } catch (e) {
    error.value = String(e)
  } finally {
    if (version === requestVersion) querying.value = false
  }
}

async function resetLazyView() {
  if (!viewRoot.value) return
  loadedDirectories.clear()
  directoryOffsets.clear()
  partialDirectories.value = new Set()
  result.value = { nodes: [{ ...viewRoot.value, parentId: -1 }], matches: 0, truncated: false, queryMs: 0 }
  await loadDirectory(viewRoot.value)
}

async function loadDirectory(node: TreeNode, loadMore = false) {
  const continuing = loadMore || (loadedDirectories.has(node.id) && partialDirectories.value.has(node.id))
  if (node.kind !== 'directory' || (!continuing && loadedDirectories.has(node.id)) || loadingDirectories.has(node.id)) return
  loadingDirectories.add(node.id)
  try {
    const offset = continuing ? (directoryOffsets.get(node.id) || 0) : 0
    const page = await LoadChildren({ parentId: node.id, text: search.value, extensions: selectedExtensions.value, offset, limit: 800 }) as { nodes: TreeNode[]; total: number; hasMore: boolean }
    const existing = new Map(result.value.nodes.map(item => [item.id, item]))
    for (const child of page.nodes || []) existing.set(child.id, child)
    const parent = existing.get(node.id)
    const nextOffset = offset + (page.nodes?.length || 0)
    if (parent) existing.set(node.id, { ...parent, hiddenMatches: Math.max(0, page.total - nextOffset) })
    result.value = { ...result.value, nodes: [...existing.values()], truncated: result.value.truncated || page.hasMore }
    directoryOffsets.set(node.id, nextOffset)
    const partial = new Set(partialDirectories.value)
    if (page.hasMore) partial.add(node.id); else partial.delete(node.id)
    partialDirectories.value = partial
    loadedDirectories.add(node.id)
  } catch (e) { error.value = String(e) }
  finally { loadingDirectories.delete(node.id) }
}

async function loadMoreFromMenu() {
  const node = contextMenu.value?.node
  contextMenu.value = null
  if (node) await loadDirectory(node, true)
}

function showContext(node: TreeNode, x: number, y: number) {
  const actionCount = 3 + (node.kind === 'directory' ? 1 : 0) + (partialDirectories.value.has(node.id) ? 1 : 0)
  const menuHeight = 48 + actionCount * 35
  contextMenu.value = {
    node,
    x: Math.max(8, Math.min(x, window.innerWidth - 210)),
    y: Math.max(8, Math.min(y, window.innerHeight - menuHeight - 8)),
  }
}

async function onlyBranch() {
  const node = contextMenu.value?.node
  contextMenu.value = null
  if (!node) return
  search.value = ''
  selectedExtensions.value = []
  branchFocusID.value = node.id
  loadedDirectories.clear()
  directoryOffsets.clear()
  partialDirectories.value = new Set()
  await runQuery()
}

async function setAsRoot() {
  const node = contextMenu.value?.node
  contextMenu.value = null
  if (!node || node.kind !== 'directory') return
  search.value = ''
  selectedExtensions.value = []
  branchFocusID.value = null
  viewRoot.value = node
  loadedDirectories.clear()
  directoryOffsets.clear()
  partialDirectories.value = new Set()
  await runQuery()
}

async function restoreFullRoot() {
  const root = await RootNode() as TreeNode
  viewRoot.value = root
  branchFocusID.value = null
  search.value = ''
  selectedExtensions.value = []
  await runQuery()
}

function scheduleQuery() {
  window.clearTimeout(searchTimer)
  searchTimer = window.setTimeout(runQuery, 110)
}

function applyDepth(depth: number) {
  maxDepth.value = depth
  depthRevision.value++
}

function toggleExtension(ext: string) {
  const values = selectedExtensions.value
  selectedExtensions.value = values.includes(ext)
    ? values.filter(value => value !== ext)
    : [...values, ext]
  runQuery()
}

function clearTypes() {
  selectedExtensions.value = []
  runQuery()
}

async function openNode(node: TreeNode) {
  try { await OpenPath(node.path) } catch (e) { error.value = String(e) }
}

async function openNodeInExplorer() {
  const node = contextMenu.value?.node
  contextMenu.value = null
  if (!node) return
  try { await OpenInExplorer(node.path) } catch (e) { error.value = String(e) }
}

async function copyNodePath() {
  const node = contextMenu.value?.node
  contextMenu.value = null
  if (!node) return
  try {
    await CopyPath(node.path)
    notice.value = '已复制绝对路径'
    window.clearTimeout(noticeTimer)
    noticeTimer = window.setTimeout(() => { notice.value = '' }, 1800)
  } catch (e) { error.value = String(e) }
}

function closeTypePanel(event: MouseEvent) {
  const target = event.target as HTMLElement
  if (!target.closest('.type-control')) showTypes.value = false
  if (!target.closest('.node-context-menu')) contextMenu.value = null
}

nextTick(() => window.addEventListener('click', closeTypePanel))
</script>

<template>
  <main class="app-shell">
    <header class="topbar">
      <div class="brand">
        <span class="brand-mark"><i></i><i></i><i></i></span>
        <span>Branchopus</span>
      </div>

      <div class="folder-selector">
        <button class="folder-switch" :disabled="scanning" @click="openFolderPicker">
          <span>↻</span>{{ summary ? '切换目录' : '选择目录' }}
        </button>
        <span v-if="summary" class="current-folder" :title="summary.root">{{ summary.rootName }}</span>
      </div>

      <div v-if="summary" class="tools">
        <label class="search-box">
          <span>⌕</span>
          <input v-model="search" :disabled="!summary" placeholder="搜索文件或路径" @input="scheduleQuery" />
          <kbd v-if="querying">···</kbd>
          <button v-else-if="search" @click="search = ''; runQuery()">×</button>
        </label>

        <div class="type-control">
          <button class="type-button" :disabled="!summary" @click.stop="showTypes = !showTypes">
            <span class="type-dot"></span>{{ selectedLabel }}<span class="down">⌄</span>
          </button>
          <div v-if="showTypes" class="type-panel" @click.stop>
            <div class="type-panel-head"><b>文件类型</b><button @click="clearTypes">全部</button></div>
            <div class="type-list">
              <label v-for="item in summary?.extensions" :key="item.extension">
                <input type="checkbox" :checked="selectedExtensions.includes(item.extension)" @change="toggleExtension(item.extension)" />
                <span>{{ item.extension }}</span><em>{{ item.count }}</em>
              </label>
            </div>
          </div>
        </div>
        <div class="depth-control" title="设置默认展开层数，之后仍可点击节点继续展开">
          <span>展开</span>
          <button v-for="depth in 5" :key="depth" @click="applyDepth(depth)">{{ depth }}</button>
          <button @click="applyDepth(0)">全</button>
        </div>
      </div>
    </header>

    <section class="workspace">
      <div v-if="!summary" class="empty-state">
        <div class="empty-tree">
          <span class="root-dot"></span><span class="line l1"></span><span class="leaf a"></span>
          <span class="line l2"></span><span class="leaf b"></span><span class="line l3"></span><span class="leaf c"></span>
        </div>
        <h1>看清文件，找到路径</h1>
        <p>将本地目录转换为清晰的分叉树，文件放在哪儿，一眼就能看到。</p>
        <button @click="openFolderPicker">选择文件夹</button>
      </div>

      <template v-else>
        <TreeCanvas :nodes="result.nodes" :auto-expand="true" :max-depth="maxDepth" :revision="treeRevision" :depth-revision="depthRevision" :filtered="Boolean(search || selectedExtensions.length)" @open="openNode" @expand="loadDirectory" @context="showContext" />
        <button v-if="viewRoot && (viewRoot.id !== 0 || branchFocusID !== null)" class="restore-root" @click="restoreFullRoot">← 返回完整目录</button>
        <div class="canvas-hint">拖动平移 · 滚轮缩放 · 双击打开</div>
        <div v-if="result.truncated" class="limit-notice">结果较多，已优先展示完整文件路径；点击带 +数量 的节点可继续加载</div>
      </template>
      <div v-if="scanning" class="scan-overlay">
        <div class="scan-card">
          <div class="scan-spinner"><i></i><i></i><i></i></div>
          <b>正在建立目录索引</b>
          <span>读取名称和层级，整理目录结构</span>
          <small>{{ pickerPath }}</small>
        </div>
      </div>
    </section>

    <footer v-if="summary" class="statusbar">
      <span><i :class="{ busy: querying || scanning }"></i>{{ querying ? '正在查询' : '索引就绪' }}</span>
      <span>{{ summary.directories.toLocaleString() }} 个目录</span>
      <span>{{ summary.files.toLocaleString() }} 个文件</span>
      <span>显示 {{ result.nodes.length.toLocaleString() }} 个节点</span>
      <span class="status-right">{{ result.matches.toLocaleString() }} 个匹配 · {{ result.queryMs }} ms</span>
    </footer>

    <div v-if="pickerOpen" class="picker-backdrop" @mousedown.self="pickerOpen = false">
      <section class="folder-picker" role="dialog" aria-modal="true" aria-label="选择文件夹">
        <header>
          <div><b>选择文件夹</b><small>按层加载，不扫描文件</small></div>
          <button class="picker-close" @click="pickerOpen = false">×</button>
        </header>
        <div class="picker-pathbar">
          <button :disabled="pickerLoading" title="磁盘列表" @click="browseFolder('')">⌂</button>
          <button :disabled="!pickerParent || pickerLoading" title="上一级" @click="browseFolder(pickerParent)">←</button>
          <input v-model="pickerPath" spellcheck="false" placeholder="输入文件夹路径" @keyup.enter="browseFolder(pickerPath)" />
          <button :disabled="pickerLoading" @click="browseFolder(pickerPath)">前往</button>
        </div>
        <div class="folder-list" :class="{ loading: pickerLoading }">
          <div v-if="pickerLoading" class="picker-message"><i></i>正在读取这一层…</div>
          <button v-for="folder in pickerFolders" v-else :key="folder.path" @click="browseFolder(folder.path)">
            <span class="mini-folder"></span><span>{{ folder.name }}</span><em>›</em>
          </button>
          <div v-if="!pickerLoading && !pickerFolders.length" class="picker-message">当前目录没有子文件夹</div>
        </div>
        <p v-if="pickerTruncated" class="picker-warning">子目录较多，仅显示前 500 个；可直接在上方输入路径。</p>
        <footer>
          <span>{{ pickerPath || '请选择一个磁盘' }}</span>
          <button class="picker-cancel" @click="pickerOpen = false">取消</button>
          <button class="picker-confirm" :disabled="!pickerPath || pickerLoading" @click="chooseFolder()">选择当前文件夹</button>
        </footer>
      </section>
    </div>
    <div v-if="error" class="error-toast" @click="error = ''">{{ error }}</div>
    <div v-if="notice" class="notice-toast">{{ notice }}</div>
    <div v-if="contextMenu" class="node-context-menu" :style="{ left: contextMenu.x + 'px', top: contextMenu.y + 'px' }" @click.stop>
      <small>{{ contextMenu.node.name }}</small>
      <button @click="openNodeInExplorer">在文件管理器中打开</button>
      <button @click="copyNodePath">复制绝对路径</button>
      <button @click="onlyBranch">仅查看该分支</button>
      <button v-if="contextMenu.node.kind === 'directory'" @click="setAsRoot">以此作为根节点</button>
      <button v-if="contextMenu.node.kind === 'directory' && partialDirectories.has(contextMenu.node.id)" @click="loadMoreFromMenu">加载更多子节点</button>
    </div>
  </main>
</template>
