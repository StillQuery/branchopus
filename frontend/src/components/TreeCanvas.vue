<script lang="ts">
export interface TreeNode {
  id: number
  parentId: number
  name: string
  path: string
  extension: string
  kind: 'directory' | 'file'
  depth: number
  matched: boolean
  hiddenMatches: number
  leafCount: number
}
</script>

<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'

const props = defineProps<{ nodes: TreeNode[]; autoExpand: boolean; maxDepth: number; revision: number; depthRevision: number; filtered: boolean }>()
const emit = defineEmits<{ open: [node: TreeNode]; expand: [node: TreeNode]; context: [node: TreeNode, x: number, y: number] }>()
const canvas = ref<HTMLCanvasElement | null>(null)

interface PlacedNode extends TreeNode { x: number; y: number; hitWidth: number }
let placed: PlacedNode[] = []
let byID = new Map<number, PlacedNode>()
let ctx: CanvasRenderingContext2D | null = null
let frame = 0
let width = 0
let height = 0
let dpr = 1
let scale = 1
let offsetX = 72
let offsetY = 70
let dragging = false
let moved = false
let lastX = 0
let lastY = 0
let hoverID = -1
let resizeObserver: ResizeObserver | null = null
let expanded = new Set<number>()
let depthPresetPending = true
let queryExpandPending = false

function layout() {
  const children = new Map<number, TreeNode[]>()
  for (const node of props.nodes) {
    if (!children.has(node.parentId)) children.set(node.parentId, [])
    children.get(node.parentId)!.push(node)
  }
  for (const list of children.values()) {
    list.sort((a, b) => a.kind === b.kind ? a.name.localeCompare(b.name, 'zh-CN') : a.kind === 'directory' ? -1 : 1)
  }

  placed = []
  byID = new Map()
  const root = props.nodes.find(node => node.parentId < 0)
  if (!root) return
  if (queryExpandPending) {
    // Expand only branches that actually lead to a visible result. A folder
    // that is itself the deepest match stays closed instead of suggesting
    // that its unrelated children are part of the search result.
    expanded = new Set(props.nodes
      .filter(node => node.kind === 'directory' && (children.get(node.id)?.length || 0) > 0)
      .map(node => node.id))
    queryExpandPending = false
    depthPresetPending = false
  } else if (depthPresetPending) {
    expanded = new Set()
    const applyPreset = (node: TreeNode, visualDepth: number) => {
      if (node.kind === 'directory' && (props.maxDepth <= 0 || visualDepth < props.maxDepth - 1)) expanded.add(node.id)
      for (const child of children.get(node.id) || []) applyPreset(child, visualDepth + 1)
    }
    applyPreset(root, 0)
    depthPresetPending = false
  }
  let leafY = 0
  const visit = (node: TreeNode, visualDepth = 0): number => {
    const kids = expanded.has(node.id) ? (children.get(node.id) || []) : []
    let y: number
    if (kids.length === 0) {
      y = leafY * 34
      leafY++
    } else {
      const ys = kids.map(child => visit(child, visualDepth + 1))
      y = (ys[0] + ys[ys.length - 1]) / 2
    }
    const item = { ...node, x: visualDepth * 218, y, hitWidth: Math.min(node.name.length, 25) * 8 + 35 }
    placed.push(item)
    byID.set(item.id, item)
    return y
  }
  visit(root)
  fitToView()
}

function fitToView() {
  if (!placed.length || !width || !height) return
  const maxX = Math.max(...placed.map(n => n.x)) + 190
  const maxY = Math.max(...placed.map(n => n.y)) + 40
  scale = Math.min(1, Math.max(0.22, Math.min((width - 100) / Math.max(maxX, 1), (height - 100) / Math.max(maxY, 1))))
  offsetX = 58
  const root = placed.find(node => node.parentId < 0)
  offsetY = root ? height / 2 - root.y * scale : 52
  drawSoon()
}

function resize() {
  const el = canvas.value
  if (!el) return
  const rect = el.getBoundingClientRect()
  width = rect.width
  height = rect.height
  dpr = Math.min(window.devicePixelRatio || 1, 2)
  el.width = Math.round(width * dpr)
  el.height = Math.round(height * dpr)
  ctx = el.getContext('2d')
  fitToView()
}

function drawSoon() {
  if (frame) return
  frame = requestAnimationFrame(() => { frame = 0; draw() })
}

function draw() {
  if (!ctx) return
  ctx.setTransform(dpr, 0, 0, dpr, 0, 0)
  ctx.clearRect(0, 0, width, height)
  ctx.save()
  ctx.translate(offsetX, offsetY)
  ctx.scale(scale, scale)
  const left = -offsetX / scale - 220
  const right = (width - offsetX) / scale + 220
  const top = -offsetY / scale - 60
  const bottom = (height - offsetY) / scale + 60

  ctx.lineWidth = 1.35 / scale
  ctx.strokeStyle = '#cbd8d3'
  ctx.beginPath()
  for (const node of placed) {
    if (node.parentId < 0) continue
    const parent = byID.get(node.parentId)
    if (!parent) continue
    // Cull by the complete edge bounds, not just the child endpoint. The
    // Bezier control points stay between the two endpoints, so this box
    // safely covers the whole curve. If it intersects the viewport, some
    // part of the line can still be visible and must be drawn.
    const edgeLeft = Math.min(parent.x + 8, node.x - 8)
    const edgeRight = Math.max(parent.x + 8, node.x - 8)
    const edgeTop = Math.min(parent.y, node.y)
    const edgeBottom = Math.max(parent.y, node.y)
    if (edgeRight < left || edgeLeft > right || edgeBottom < top || edgeTop > bottom) continue
    const midX = parent.x + (node.x - parent.x) * 0.52
    ctx.moveTo(parent.x + 8, parent.y)
    ctx.bezierCurveTo(midX, parent.y, midX, node.y, node.x - 8, node.y)
  }
  ctx.stroke()

  for (const node of placed) {
    if (node.x < left || node.x > right || node.y < top || node.y > bottom) continue
    const hovered = node.id === hoverID
    const isDir = node.kind === 'directory'
    ctx.beginPath()
    ctx.arc(node.x, node.y, hovered ? 6.5 : 5, 0, Math.PI * 2)
    ctx.fillStyle = node.matched ? '#d77752' : isDir ? '#356f63' : '#94a39e'
    ctx.fill()
    if (hovered) {
      ctx.lineWidth = 3 / scale
      ctx.strokeStyle = 'rgba(53,111,99,.22)'
      ctx.stroke()
    }

    const label = shorten(node.name, scale < .55 ? 16 : 25)
    ctx.font = `${node.matched ? 600 : 500} ${Math.max(11, 13 / Math.max(scale, .72))}px "Segoe UI", "Microsoft YaHei", sans-serif`
    ctx.textBaseline = 'middle'
    const textWidth = ctx.measureText(label).width
    node.hitWidth = textWidth + 31
    if (hovered || node.matched) {
      ctx.fillStyle = hovered ? '#ffffff' : '#fff0e9'
      roundRect(ctx, node.x + 11, node.y - 13, textWidth + 13, 26, 7)
      ctx.fill()
    }
    ctx.fillStyle = node.matched ? '#a84f30' : '#2f3d39'
    ctx.fillText(label, node.x + 17, node.y + .5)
    if (node.kind === 'directory' && node.leafCount > 0) {
      const badge = props.filtered ? `${node.leafCount.toLocaleString()} 命中` : `${node.leafCount.toLocaleString()} 文件`
      const badgeX = node.x + 24 + textWidth
      ctx.font = `600 ${Math.max(9, 10 / Math.max(scale, .72))}px "Segoe UI", sans-serif`
      const badgeWidth = ctx.measureText(badge).width + 10
      node.hitWidth += badgeWidth + 7
      ctx.fillStyle = '#e8f2ef'
      roundRect(ctx, badgeX, node.y - 9, badgeWidth, 18, 9)
      ctx.fill()
      ctx.fillStyle = '#547a70'
      ctx.fillText(badge, badgeX + 5, node.y + .5)
      if (node.hiddenMatches > 0) {
        const moreX = badgeX + badgeWidth + 5
        ctx.fillStyle = '#82918b'
        ctx.fillText('•••', moreX, node.y - 1)
        node.hitWidth += 19
      }
    }
  }
  ctx.restore()
}

function roundRect(context: CanvasRenderingContext2D, x: number, y: number, w: number, h: number, r: number) {
  context.beginPath(); context.roundRect(x, y, w, h, r)
}

function shorten(value: string, max: number) { return value.length > max ? value.slice(0, max - 1) + '…' : value }

function worldPoint(event: MouseEvent) {
  const rect = canvas.value!.getBoundingClientRect()
  return { x: (event.clientX - rect.left - offsetX) / scale, y: (event.clientY - rect.top - offsetY) / scale }
}

function hitTest(event: MouseEvent) {
  const p = worldPoint(event)
  const verticalRadius = Math.max(12, 12 / scale)
  for (const node of placed) {
    if (Math.abs(node.y - p.y) > verticalRadius) continue
    if (p.x >= node.x - 12 && p.x <= node.x + node.hitWidth) return node
  }
  return undefined
}

function onPointerDown(event: PointerEvent) {
  dragging = true; moved = false; lastX = event.clientX; lastY = event.clientY
  canvas.value?.setPointerCapture(event.pointerId)
}

function onPointerMove(event: PointerEvent) {
  if (dragging) {
    const dx = event.clientX - lastX, dy = event.clientY - lastY
    if (Math.abs(dx) + Math.abs(dy) > 2) moved = true
    offsetX += dx; offsetY += dy; lastX = event.clientX; lastY = event.clientY; drawSoon()
  } else {
    const hit = hitTest(event)
    const next = hit?.id ?? -1
    if (next !== hoverID) { hoverID = next; if (canvas.value) canvas.value.style.cursor = hit ? 'pointer' : 'grab'; drawSoon() }
  }
}

function onPointerUp() { dragging = false; if (canvas.value) canvas.value.style.cursor = 'grab' }

function onClick(event: MouseEvent) {
  if (moved) return
  const node = hitTest(event)
  if (!node || node.kind !== 'directory') return
  if (expanded.has(node.id) && node.hiddenMatches > 0) { emit('expand', node); return }
  if (expanded.has(node.id)) expanded.delete(node.id)
  else { expanded.add(node.id); emit('expand', node) }
  layout()
}

function onContextMenu(event: MouseEvent) {
  event.preventDefault()
  const node = hitTest(event)
  if (node) emit('context', node, event.clientX, event.clientY)
}

function onWheel(event: WheelEvent) {
  event.preventDefault()
  const rect = canvas.value!.getBoundingClientRect()
  const px = event.clientX - rect.left, py = event.clientY - rect.top
  const wx = (px - offsetX) / scale, wy = (py - offsetY) / scale
  const next = Math.min(2.5, Math.max(.12, scale * Math.exp(-event.deltaY * .0012)))
  offsetX = px - wx * next; offsetY = py - wy * next; scale = next; drawSoon()
}

function onDoubleClick(event: MouseEvent) {
  if (moved) return
  const node = hitTest(event)
  if (node) emit('open', node)
}

watch(() => props.nodes, async () => {
  const ids = new Set(props.nodes.map(node => node.id))
  expanded = new Set([...expanded].filter(id => ids.has(id)))
  await nextTick(); layout()
}, { deep: false })
watch(() => props.autoExpand, () => { depthPresetPending = true; layout() })
watch(() => props.depthRevision, () => { depthPresetPending = true; layout() })
watch(() => props.revision, () => { queryExpandPending = true; layout() })
onMounted(() => {
  resizeObserver = new ResizeObserver(resize)
  if (canvas.value) resizeObserver.observe(canvas.value)
  layout()
})
onBeforeUnmount(() => { resizeObserver?.disconnect(); if (frame) cancelAnimationFrame(frame) })
</script>

<template>
  <canvas ref="canvas" class="tree-canvas" @pointerdown="onPointerDown" @pointermove="onPointerMove"
    @pointerup="onPointerUp" @pointercancel="onPointerUp" @click="onClick" @wheel="onWheel" @dblclick="onDoubleClick" @contextmenu="onContextMenu" />
</template>
