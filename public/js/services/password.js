scionApp
    .factory('passwordService', ["$http", "$q", function ($http, $q) {
    return {
        setPassword: function (user) {
            return $http.post('/api/setPassword', user).then(function (response) {
                console.log(response);
                return response.data;
            });
        }
    };
}]);
