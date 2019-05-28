export function lineChart(tagId, frame, dataFeed) {
	let margin = {top:10, right:30, bottom:30, left:60}

	let svg = d3.select(tagId)
			.append("svg")
				.attr("width", frame.width+margin.left+margin.right)
				.attr("height", frame.height+margin.top+margin.bottom)
			.append("g")
				.attr("transform", `translate(${margin.left},${margin.top})`)

	d3.csv(dataFeed, function(d){
		let parts = d.date.split("-")
		return {
			date: new Date(parts[0], parts[1], parts[2]), 
			value: +d.value
		}
	}).then(function (data) {
		let scales = {
			x: d3.scaleTime()
				.domain(d3.extent(data, d => d.date))
				.range([0, frame.width]),
			y: d3.scaleLinear()
				.domain([0, d3.max(data, d => d.value)])
				.range([frame.height, 0])
		}

		// x-axis
		svg.append("g")
			.attr("transform", `translate(0,${frame.height})`)
			.call(d3.axisBottom(scales.x))
		
		// y-axis
		svg.append("g")
			.call(d3.axisLeft(scales.y))

		let line = d3.line()
			.x(d => scales.x(d.date))
			.y(d => scales.y(d.value))

		// path
		svg.append("path")
			.attr("fill", "none")
			.attr("stroke", "steelblue")
			.attr("stroke-width", 1.5)
			.attr("d", line(data))
	})
}

