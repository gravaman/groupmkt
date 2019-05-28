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
				.attr("width", frame.width+margin.left+margin.right)
				.attr("height", frame.height+margin.top+margin.bottom)
			.append("g")
				.attr("transform", `translate(${margin.left},${margin.top})`)

	// pull historical data
	let data = priorData(ticker, freq, interval)
	let xrng = d3.range(0,data.length,1)

	console.log("historical data...\n", data)
	console.log("xrng:...\n", xrng)

	// set scales
	let scales = {
		x: d3.scaleTime()
			.domain([0,xrng.length-1])
			.range([0, frame.width]),
		y: d3.scaleLinear()
			.domain([d3.min(data, d => d.value), d3.max(data, d => d.value)])
			.range([frame.height, 0])
	}

	// x-axis
	svg.append("g")
		.attr("transform", `translate(0,${frame.height})`)
		.call(d3.axisBottom(scales.x))
		
	// y-axis
	svg.append("g")
		.call(d3.axisLeft(scales.y))

	// line path
	let line = d3.line()
			.x((d,i) => scales.x(i))
			.y(d => scales.y(d.value))

	let linePath = svg.append("path")
				.attr("fill", "none")
				.attr("stroke", "steelblue")
				.attr("stroke-width", 1.5)
				.attr("d", line(data))

	// redraw graph at given frequency 
	setInterval(redraw, 1000)

	function redraw() {
		data.shift()
		data.push(pullData(ticker))

		linePath.attr("d", line(data))
			.transition()
			.duration(d3.easeLinear(1000))
	}
}

