import { lineChart } from '/js/lineChart.js'

$(document).ready(function() {
	// main visual setup
	let frame = {
		width: Math.min($('#main-display-visual').parent().width(), 900),
	}
	frame.height = frame.width*$(window).height()/$(window).width()
	lineChart("#main-display-visual", frame, "data/linechart_data.csv")

	// click handlers
	$('#sidebarCollapse').on('click', function() {
		$('#sidebar').addClass('inactive')
		$('.overlay').addClass('active')
	})
	$('#dismiss, .overlay').on('click', function() {
		$('#sidebar').removeClass('inactive')
		$('.overlay').removeClass('active')
	})
})

