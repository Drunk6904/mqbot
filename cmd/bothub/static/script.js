/* ========== 状态 ========== */
const robots = {} // { botId: { id, x, y, battery, state, speed, ts } }
let selectedBot = null
let ws = null

/* ========== WebSocket ========== */
const WS_URL = `${location.protocol === "https:" ? "wss" : "ws"}://${location.host}/ws`

function connectWS() {
	ws = new WebSocket(WS_URL)

	ws.onopen = () => {
		document.getElementById("wsDot").classList.add("connected")
		document.getElementById("wsLabel").textContent = "已连接"
	}

	ws.onclose = () => {
		document.getElementById("wsDot").classList.remove("connected")
		document.getElementById("wsLabel").textContent = "已断开"
		setTimeout(connectWS, 3000) // 自动重连
	}

	ws.onerror = () => ws.close()

	ws.onmessage = (e) => {
		try {
			const d = JSON.parse(e.data)
			if (d.id) {
				robots[d.id] = { ...d, ts: Date.now() }
				renderList()
			}
		} catch (_) {}
	}
}

connectWS()

/* ========== 机器人列表 ========== */
function renderList() {
	const container = document.getElementById("robotCards")
	const ids = Object.keys(robots)

	if (ids.length === 0) {
		container.innerHTML = '<div class="robot-list__empty">等待机器人连接...</div>'
		return
	}

	container.innerHTML = ids
		.map((id) => {
			const r = robots[id]
			const batteryColor = r.battery > 50 ? "var(--green)" : r.battery > 20 ? "var(--yellow)" : "var(--red)"
			const sel = selectedBot === id ? " selected" : ""
			return `
      <div class="robot-card${sel}" onclick="selectBot('${id}')">
        <div class="robot-card__header">
          <span class="robot-card__id">${esc(id)}</span>
          <span class="robot-card__state robot-card__state--${r.state || "OFFLINE"}">${r.state || "OFFLINE"}</span>
        </div>
        <div class="robot-card__coords">x: ${r.x.toFixed(2)} &nbsp; y: ${r.y.toFixed(2)}</div>
        <div class="robot-card__battery">
          <div class="robot-card__battery-bar">
            <div class="robot-card__battery-fill" style="width:${r.battery}%;background:${batteryColor}"></div>
          </div>
          <span class="robot-card__battery-text">${r.battery}%</span>
        </div>
      </div>`
		})
		.join("")
}

function selectBot(id) {
	selectedBot = id
	document.getElementById("selectedInfo").textContent = `目标: ${id}`
	document.getElementById("btnSend").disabled = false
	renderList()
}

function esc(s) {
	const d = document.createElement("div")
	d.textContent = s
	return d.innerHTML
}

/* ========== 任务下发 ========== */
document.getElementById("btnSend").addEventListener("click", () => {
	if (!selectedBot || !ws || ws.readyState !== WebSocket.OPEN) return

	const x = document.getElementById("targetX").value
	const y = document.getElementById("targetY").value
	if (!x && !y) return

	ws.send(
		JSON.stringify({
			bot_id: selectedBot,
			action: "move_to",
			params: { x: String(x || "0"), y: String(y || "0") },
		}),
	)
})

/* ========== Canvas 画布 ========== */
const canvas = document.getElementById("map")
const ctx = canvas.getContext("2d")
const GRID = 40 // 坐标范围 -20 ~ 20
const STEP = 5

let mouseX = null,
	mouseY = null

function resize() {
	const rect = canvas.parentElement.getBoundingClientRect()
	canvas.width = rect.width * devicePixelRatio
	canvas.height = rect.height * devicePixelRatio
	canvas.style.width = rect.width + "px"
	canvas.style.height = rect.height + "px"
	ctx.setTransform(devicePixelRatio, 0, 0, devicePixelRatio, 0, 0)
}
window.addEventListener("resize", resize)
resize()

// 坐标转换：世界坐标 → 画布像素
function toCanvas(wx, wy) {
	const rect = canvas.getBoundingClientRect()
	const w = rect.width,
		h = rect.height
	const cx = w / 2,
		cy = h / 2
	const scale = Math.min(w, h) / GRID
	return [cx + wx * scale, cy - wy * scale]
}

// 画布像素 → 世界坐标
function toWorld(px, py) {
	const rect = canvas.getBoundingClientRect()
	const w = rect.width,
		h = rect.height
	const cx = w / 2,
		cy = h / 2
	const scale = Math.min(w, h) / GRID
	return [(px - cx) / scale, (cy - py) / scale]
}

// 鼠标坐标提示
canvas.addEventListener("mousemove", (e) => {
	const rect = canvas.getBoundingClientRect()
	const [wx, wy] = toWorld(e.clientX - rect.left, e.clientY - rect.top)
	mouseX = wx
	mouseY = wy
	document.getElementById("canvasInfo").textContent = `x: ${wx.toFixed(1)}  y: ${wy.toFixed(1)}`
})
canvas.addEventListener("mouseleave", () => {
	document.getElementById("canvasInfo").textContent = "移动鼠标查看坐标"
})

// 点击画布设置目标坐标
canvas.addEventListener("click", (e) => {
	const rect = canvas.getBoundingClientRect()
	const [wx, wy] = toWorld(e.clientX - rect.left, e.clientY - rect.top)
	document.getElementById("targetX").value = wx.toFixed(1)
	document.getElementById("targetY").value = wy.toFixed(1)
})

// 状态颜色
const STATE_COLORS = {
	IDLE: "#3fb950",
	MOVING: "#58a6ff",
	CHARGING: "#d29922",
	ERROR: "#f85149",
	OFFLINE: "#484f58",
}

// 绘制
function draw() {
	const rect = canvas.getBoundingClientRect()
	const w = rect.width,
		h = rect.height
	ctx.clearRect(0, 0, w, h)

	// -- 网格线 --
	ctx.strokeStyle = "#161b22"
	ctx.lineWidth = 1
	for (let i = -GRID / 2; i <= GRID / 2; i += STEP) {
		// 竖线
		const [x1, y1] = toCanvas(i, -GRID / 2)
		const [x2, y2] = toCanvas(i, GRID / 2)
		ctx.beginPath()
		ctx.moveTo(x1, y1)
		ctx.lineTo(x2, y2)
		ctx.stroke()
		// 横线
		const [x3, y3] = toCanvas(-GRID / 2, i)
		const [x4, y4] = toCanvas(GRID / 2, i)
		ctx.beginPath()
		ctx.moveTo(x3, y3)
		ctx.lineTo(x4, y4)
		ctx.stroke()
	}

	// -- 坐标轴 --
	const [ox, oy] = toCanvas(0, 0)
	ctx.strokeStyle = "#30363d"
	ctx.lineWidth = 1.5
	// X 轴
	ctx.beginPath()
	ctx.moveTo(0, oy)
	ctx.lineTo(w, oy)
	ctx.stroke()
	// Y 轴
	ctx.beginPath()
	ctx.moveTo(ox, 0)
	ctx.lineTo(ox, h)
	ctx.stroke()

	// 轴标签
	ctx.font = '11px "JetBrains Mono"'
	ctx.fillStyle = "#484f58"
	ctx.textAlign = "center"
	for (let i = -GRID / 2; i <= GRID / 2; i += STEP) {
		if (i === 0) continue
		const [lx] = toCanvas(i, 0)
		ctx.fillText(i, lx, oy + 14)
		const [, ly] = toCanvas(0, i)
		ctx.fillText(i, ox - 14, ly + 4)
	}

	// -- 原点标记 --
	ctx.fillStyle = "#484f58"
	ctx.textAlign = "right"
	ctx.fillText("O", ox - 6, oy + 14)

	// -- 机器人 --
	for (const id in robots) {
		const r = robots[id]
		const [px, py] = toCanvas(r.x, r.y)
		const color = STATE_COLORS[r.state] || STATE_COLORS.OFFLINE

		// 光晕
		const glow = ctx.createRadialGradient(px, py, 0, px, py, 16)
		glow.addColorStop(0, color + "40")
		glow.addColorStop(1, "transparent")
		ctx.fillStyle = glow
		ctx.beginPath()
		ctx.arc(px, py, 16, 0, Math.PI * 2)
		ctx.fill()

		// 圆点
		ctx.fillStyle = color
		ctx.beginPath()
		ctx.arc(px, py, 5, 0, Math.PI * 2)
		ctx.fill()

		// 选中环
		if (selectedBot === id) {
			ctx.strokeStyle = "#58a6ff"
			ctx.lineWidth = 1.5
			ctx.beginPath()
			ctx.arc(px, py, 10, 0, Math.PI * 2)
			ctx.stroke()
		}

		// 标签
		ctx.font = '11px "JetBrains Mono"'
		ctx.fillStyle = color
		ctx.textAlign = "left"
		ctx.fillText(id, px + 12, py + 4)
	}

	requestAnimationFrame(draw)
}
draw()
