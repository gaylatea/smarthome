// Server for water control.
const awsIot = require('aws-iot-device-sdk');
const exec   = require('child_process').exec;

// AWS SETUP ===================================================================
var device = awsIot.device({
   keyPath: 'private.pem.key',
  certPath: 'cert.pem.crt',
    caPath: 'aws.pem.key',
  clientId: 'plant-jasmine',
    region: 'us-west-2',
});

// HANDLERS ====================================================================
const dispenseWater = function(duration) {
  device.publish("Den/Barrel/cmd", JSON.stringify({"water": duration}));
}

const measureMoisture = function() {
  console.log("MEASURING");

  exec('python adc.py', (err, output, stderr) => {
    console.log(output);
    device.publish("Den/Jasmine/measurements", output);
  });
};

device.on('connect', function() {
  console.log('connect');

  // Start measuring plant moisture.
  setImmediate(measureMoisture);
  setInterval(measureMoisture, (10 * 60 * 1000));
  });

// END =========================================================================
