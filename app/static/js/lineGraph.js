/* jshint esversion: 6 */
function pullData(ticker) {
	// returns data with given ticker, current datetime, and pseudorandom value uniformly
	// distributed over range [99, 102)
	return {
		ticker: ticker,
		date: new Date(),
		value: Math.random()*3+99
	}
}

function priorData(ticker, freq, interval) {
	// returns historical data for given ticker over prior interval with given frequency
	// freq is the data frequency during the interval in seconds
	// interval is the data period interval in seconds
	let data = []
	let end = new Date()
	let last = end-interval*1000
	while (last < end) {
		data.push({
			ticker: ticker,
			date: new Date(last),
			value: Math.random()*3+99
		})
		last += freq*1000
	}
	return data
}

export function lineGraph(tagId, frame, ticker, freq, interval) {
	let margin = {top:10, right:30, bottom:30, left:60}

	let svg = d3.select(tagId)
			.append("svg")
				.attr("class", "line-graph")
				.attr("width", frame.width+margin.left+margin.right)
				.attr("height", frame.height+margin.top+margin.bottom)
			.append("g")
				.attr("transform", `translate(${margin.left},${margin.top})`)

	// content background
	svg.append("rect")
		.attr("width", frame.width+2) 
		.attr("height", frame.height+2)
		.attr("x", -1)
		.attr("y",-1)
		.style("fill", "#eaeaea")

	// pull historical data
	let data = priorData(ticker, freq, interval)
	let xrng = d3.range(0,data.length,1)
	let bounds = [d3.min(data, d => d.value), d3.max(data, d => d.value)]

	// set scales
	let scales = {
		x: d3.scaleLinear()
			.domain([0,xrng.length-1])
			.range([0, frame.width]),
		xAx: d3.scaleTime()
			.domain([data[0].date, data[data.length-1].date])
			.range([0,frame.width]),
		y: d3.scaleLinear()
			.domain(bounds)
			.range([frame.height, 0])
	}

	// x-axis
	scales.xAx.axis = d3.axisBottom(scales.xAx)
				.tickSizeOuter(0)
				.tickFormat(d3.timeFormat("%H:%M:%S"))

	let xAx = svg.append("g")
			.attr("class", "x-axis")
			.attr("transform", `translate(0,${frame.height})`)
			.call(scales.xAx.axis)
			
	// y-axis
	scales.y.axis = d3.axisLeft(scales.y)
				.tickSizeOuter(0)

	let yAx = svg.append("g")
			.attr("class", "y-axis")
			.call(scales.y.axis)

	// heartbeat transition on document root element
	let heartbeat = d3.transition("heartbeat")
				.duration(freq*1000)
				.ease(d3.easeLinear)

	// line graph
	let line = d3.line()
			.x((d,i) => scales.x(i))
			.y(d => scales.y(d.value))

	let linePath = svg.append("path")
				.attr("class", "line-graph-path")
				.attr("d", line(data))

	// redraw graph at given frequency 
	setInterval(tick, freq*1000)
	function tick() {
		// update data
		let v = pullData(ticker)
		data.shift()
		data.push(v)

		// update x-axis
		scales.xAx.domain([data[0].date, v.date]) 
		xAx.call(scales.xAx.axis)

		// update y-axis
		bounds = [Math.min(bounds[0], v.value), Math.max(bounds[1], v.value)]
		scales.y.domain(bounds)
		yAx.call(scales.y.axis)
		
		// update path
		linePath.attr("d", line(data))
	}
}

