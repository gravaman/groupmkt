/* jshint esversion: 6 */
import { lineGraph } from '/js/lineGraph.js'

$(document).ready(function() {
	// main visual: pull data every second over a 1 minute interval
	lineGraph("#main-display-visual", getFrame(), "FUN", 1, 60) 

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
		width: Math.min($('#main-display-visual').parent().width(), 900)
	}
	frame.height = frame.width*$(window).height()/$(window).width()
	return frame
}
