// Server for water control.
var awsIot      = require('aws-iot-device-sdk');
var usonic      = require('mmm-usonic');
var gpio        = require('mmm-gpio');
var nodeCleanup = require('node-cleanup');

// AWS SETUP ===================================================================

var device = awsIot.device({
   keyPath: 'private.pem.key',
  certPath: 'cert.pem.crt',
    caPath: 'aws.pem.key',
  clientId: 'barrel-pi',
    region: 'us-west-2'
});

// GPIO CONFIGURATION ==========================================================
var solenoidTrigger = {};
var solenoidPower = {};

gpio.init(function (error) {
  if(error) {
    console.log("Could not init sensor.");
    process.exit(-1);
  } else {
    solenoidTrigger = gpio.createOutput(2);
    solenoidPower = gpio.createOutput(3);
  }
});

// REMOTE WATERING =============================================================
var startWatering = function(duration) {
  console.log("STARTING WATERING");
  solenoidTrigger(true);
  setTimeout(stopWatering, (duration * 1000));
};

var stopWatering = function(duration) {
  console.log("STOPPING WATERING");
  solenoidTrigger(false);
};

// WATER MEASUREMENT ===========================================================
var sensor = {};

usonic.init(function (error) {
  if(error) {
    console.log("Could not init sensor.");
    process.exit(-1);
  } else {
    sensor = usonic.createSensor(17, 18);
  }
});

var measureWater = function(duration) {
  console.log("MEASURING");
  var distance = sensor();
  if(distance == -1) {
    console.log("Could not measure water. Skipping.");
    return;
  }

  // Compute a reasonable measurement of how full the barrel is.
  // We have a bunch of padding on each direction, so even the alarm threshold
  // (5% as measured here) still has some leeway before it's physically unable
  // to water anymore.
  var percent = (1.0 - ((distance - 5)/67)) * 100;
  console.log("Water at %d%%.", percent);
  device.publish("Den/Barrel/measurements", JSON.stringify({'water': percent}));
};

// HANDLERS ====================================================================
var commandHandlers = {
  'water': startWatering,
};

device.on('connect', function() {
  console.log('connect');
  device.subscribe('Den/Barrel/cmd');

  // Kick off an initial read of water, and a recurring schedule.
  setImmediate(measureWater);
  setInterval(measureWater, (10 * 60 * 1000));
  });

device.on('message', function(topic, payload) {
  var commands = JSON.parse(payload);
  console.log('message', topic, commands);

  // Each key in the given JSON object is a command to the device, so dispatch
  // as appropriate.
  Object.keys(commands).forEach(function(k){
    var v = commands[k];
    commandHandlers[k](v);
  });
});

// END =========================================================================
