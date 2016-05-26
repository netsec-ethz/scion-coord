angular.module('scionApp')
    .controller('registerCtrl', ['$scope', 'registerService', '$interval', '$location',
        function($scope, registerService, $interval, $location) {            

            $scope.error = "";
            $scope.message = "";
            $scope.user = {};

            // refresh the list of processes
            $scope.register = function (user) {
                
                registerService.register(user).then(
                    function(data) {                    
                        //$scope.message = "Registration completed successfully.\nYou will be soon redirected to the home page.";
                        $scope.user = data;
                        $location.path('/admin');
                        
                    },
                    function(response) {
                        $scope.error = "Registration error. Please try again.";
                        console.log(response);
                    });  
            };

 }]);

