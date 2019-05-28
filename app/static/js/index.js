import { lineChart } from '/js/lineChart.js'

$(document).ready(function() {
	// main visual
	lineChart("#main-display-visual", getFrame(), "data/linechart_data.csv")

	// sidebar toggle
	$('#sidebarCollapse').on('click', function() {
		$('#sidebar').addClass('inactive')
		$('.overlay').addClass('active')
	})

	$('#dismiss, .overlay').on('click', function() {
		$('#sidebar').removeClass('inactive')
		$('.overlay').removeClass('active')
	})
})

function getFrame() {
	let frame = {
		width: Math.min($('#main-display-visual').parent().width(), 90)
	}
	frame.height = frame.width*$(window).height()/$(window).width()
	return frame
}
